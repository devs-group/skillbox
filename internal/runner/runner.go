package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/sandbox"
	"github.com/devs-group/skillbox/internal/store"
)

// RunRequest describes a skill execution request.
type RunRequest struct {
	Skill    string            `json:"skill"`
	Version  string            `json:"version"`
	Input    json.RawMessage   `json:"input"`
	Env      map[string]string `json:"env,omitempty"`
	TenantID string            `json:"-"`
}

// RunResult holds the outcome of a skill execution.
type RunResult struct {
	ExecutionID string          `json:"execution_id"`
	Status      string          `json:"status"` // success, failed, timeout
	Output      json.RawMessage `json:"output,omitempty"`
	FilesURL    string          `json:"files_url,omitempty"`
	FilesList   []string        `json:"files_list,omitempty"`
	Logs        string          `json:"logs,omitempty"`
	DurationMs  int64           `json:"duration_ms"`
	Error       *string         `json:"error"`
}

// setError is a helper that sets the Error field on a RunResult from a plain string.
func (r *RunResult) setError(msg string) {
	if msg == "" {
		r.Error = nil
		return
	}
	r.Error = &msg
}

// Runner orchestrates skill execution in OpenSandbox sandboxes.
type Runner struct {
	sandbox   *sandbox.Client
	config    *config.Config
	registry  *registry.Registry
	store     *store.Store
	artifacts *artifacts.Collector
	sem       chan struct{} // concurrency limiter
}

// New creates a Runner with all required dependencies.
func New(cfg *config.Config, sb *sandbox.Client, reg *registry.Registry, st *store.Store, art *artifacts.Collector) *Runner {
	return &Runner{
		sandbox:   sb,
		config:    cfg,
		registry:  reg,
		store:     st,
		artifacts: art,
		sem:       make(chan struct{}, cfg.MaxConcurrentExecs),
	}
}

// Run executes a skill in an OpenSandbox sandbox. It handles the complete
// lifecycle: record creation, skill loading, sandbox setup, file upload,
// command execution, output collection, artifact uploading, and cleanup.
//
// The context controls the overall execution timeout. If the context is
// cancelled or times out, the sandbox is deleted and the execution is
// marked as "timeout".
func (r *Runner) Run(ctx context.Context, req RunRequest) (result *RunResult, err error) {
	// Acquire a concurrency slot (blocks if all slots are in use).
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	startTime := time.Now()

	// Resolve "latest" version to the most recently uploaded version.
	if req.Version == "" || req.Version == "latest" {
		resolved, resolveErr := r.registry.ResolveLatest(ctx, req.TenantID, req.Skill)
		if resolveErr != nil {
			if errors.Is(resolveErr, registry.ErrSkillNotFound) {
				return nil, ErrSkillNotFound
			}
			return nil, fmt.Errorf("resolving latest version for %s: %w", req.Skill, resolveErr)
		}
		req.Version = resolved
	}

	// Step 1: Create execution record in Postgres (status: running).
	exec, dbErr := r.store.CreateExecution(ctx, &store.Execution{
		SkillName:    req.Skill,
		SkillVersion: req.Version,
		TenantID:     req.TenantID,
		Input:        req.Input,
	})
	if dbErr != nil {
		return nil, fmt.Errorf("creating execution record: %w", dbErr)
	}
	executionID := exec.ID

	// Prepare the result that we will update on completion.
	result = &RunResult{
		ExecutionID: executionID,
		Status:      "failed",
	}

	// Ensure we always update the execution record in the database,
	// even if we return early due to an error.
	defer func() {
		now := time.Now()
		result.DurationMs = now.Sub(startTime).Milliseconds()

		updateExec := &store.Execution{
			ID:         executionID,
			Status:     result.Status,
			Output:     result.Output,
			Logs:       result.Logs,
			FilesURL:   result.FilesURL,
			FilesList:  result.FilesList,
			DurationMs: result.DurationMs,
			Error:      result.Error,
			FinishedAt: &now,
		}
		if updateErr := r.store.UpdateExecution(context.Background(), updateExec); updateErr != nil {
			log.Printf("runner: failed to update execution %s: %v", executionID, updateErr)
		}
	}()

	// Step 2: Load skill from registry (download, extract, validate).
	loadedSkill, err := registry.LoadSkill(ctx, r.registry, req.TenantID, req.Skill, req.Version)
	if err != nil {
		result.setError(fmt.Sprintf("loading skill: %v", err))
		return result, nil
	}
	defer func() {
		if removeErr := os.RemoveAll(loadedSkill.Dir); removeErr != nil {
			log.Printf("runner: failed to remove skill dir %s: %v", loadedSkill.Dir, removeErr)
		}
	}()

	// Step 3: Validate image against allowlist.
	image := loadedSkill.Skill.DefaultImage()
	if err := ValidateImage(image, r.config.ImageAllowlist); err != nil {
		result.setError(fmt.Sprintf("image validation: %v", err))
		return result, nil
	}

	// Determine resource limits. Use skill-level overrides or server defaults,
	// clamped to server-side maximums to prevent resource exhaustion.
	memoryStr := r.config.DefaultMemoryStr()
	if loadedSkill.Skill.Resources.Memory != "" {
		memoryStr = loadedSkill.Skill.Resources.Memory
		// Clamp to MaxMemory if the skill requests more than allowed.
		if requested, parseErr := config.ParseMemory(memoryStr); parseErr == nil && requested > r.config.MaxMemory {
			slog.Warn("clamping skill memory to server maximum",
				"skill", req.Skill, "requested", memoryStr,
				"max_bytes", r.config.MaxMemory)
			memoryStr = r.config.DefaultMemoryStr()
		}
	}

	cpuStr := r.config.DefaultCPUStr()
	if loadedSkill.Skill.Resources.CPU != "" {
		cpuStr = loadedSkill.Skill.Resources.CPU
		// Clamp to MaxCPU if the skill requests more than allowed.
		if requested, parseErr := strconv.ParseFloat(cpuStr, 64); parseErr == nil && requested > r.config.MaxCPU {
			slog.Warn("clamping skill CPU to server maximum",
				"skill", req.Skill, "requested", cpuStr,
				"max_cpu", r.config.MaxCPU)
			cpuStr = r.config.DefaultCPUStr()
		}
	}

	// Determine execution timeout.
	timeout := r.config.DefaultTimeout
	if loadedSkill.Skill.Timeout > 0 {
		timeout = min(loadedSkill.Skill.Timeout, r.config.MaxTimeout)
	}
	execCtx, execCancel := context.WithTimeout(ctx, timeout)
	defer execCancel()

	// Step 4: Prepare input JSON.
	inputJSON := req.Input
	if inputJSON == nil {
		inputJSON = json.RawMessage("{}")
	}

	// Step 5: Build environment variables, filtering blocked ones.
	envVars := map[string]string{
		"SANDBOX_INPUT":      string(inputJSON),
		"SANDBOX_OUTPUT":     "/sandbox/out/output.json",
		"SANDBOX_FILES_DIR":  "/sandbox/out/files/",
		"SKILL_INSTRUCTIONS": loadedSkill.Skill.Instructions,
		"HOME":               "/tmp",
	}
	for k, v := range req.Env {
		if isBlockedEnvVar(k) {
			result.setError(fmt.Sprintf("env var %q is not allowed", k))
			return result, nil
		}
		envVars[k] = v
	}

	// Step 6: Create OpenSandbox sandbox.
	// Convert the sandbox expiration to seconds, clamped to the API limits (60-86400).
	sandboxTimeoutSec := int(r.config.SandboxExpiration.Seconds())
	if sandboxTimeoutSec < 60 {
		sandboxTimeoutSec = 60
	}
	if sandboxTimeoutSec > 86400 {
		sandboxTimeoutSec = 86400
	}

	sbOpts := sandbox.SandboxOpts{
		Image:      image,
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Env:        envVars,
		Metadata: map[string]string{
			"managed-by": "skillbox",
			"tenant":     req.TenantID,
			"skill":      req.Skill,
			"execution":  executionID,
		},
		ResourceLimits: map[string]string{
			"cpu":    cpuStr,
			"memory": memoryStr,
		},
		NetworkPolicy: &sandbox.NetworkPolicy{
			DefaultAction: "deny",
		},
		Timeout: sandboxTimeoutSec,
	}

	sbResp, createErr := r.sandbox.CreateSandbox(execCtx, sbOpts)
	if createErr != nil {
		result.setError(fmt.Sprintf("creating sandbox: %v", createErr))
		return result, nil
	}
	sandboxID := sbResp.ID

	// Ensure sandbox is always deleted on exit.
	defer func() {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer deleteCancel()
		if deleteErr := r.sandbox.DeleteSandbox(deleteCtx, sandboxID); deleteErr != nil {
			log.Printf("runner: failed to delete sandbox %s: %v", shortID(sandboxID), deleteErr)
		}
	}()

	// Step 7: Wait for the sandbox to reach Running state and discover ExecD.
	if _, waitErr := r.sandbox.WaitReady(execCtx, sandboxID); waitErr != nil {
		result.setError(fmt.Sprintf("waiting for sandbox to become ready: %v", waitErr))
		return result, nil
	}

	execdURL, _, discoverErr := r.sandbox.DiscoverExecD(execCtx, sandboxID)
	if discoverErr != nil {
		result.setError(fmt.Sprintf("discovering execd endpoint: %v", discoverErr))
		return result, nil
	}

	// Step 8: Poll ExecD until ready (200ms interval, 30s timeout).
	if pingErr := pollExecD(execCtx, r.sandbox, execdURL, 200*time.Millisecond, 30*time.Second); pingErr != nil {
		result.setError(fmt.Sprintf("waiting for execd to become ready: %v", pingErr))
		return result, nil
	}

	// Step 9: Upload skill files + input.json to the sandbox.
	uploadFiles, walkErr := buildUploadFiles(loadedSkill.Dir, inputJSON)
	if walkErr != nil {
		result.setError(fmt.Sprintf("preparing files for upload: %v", walkErr))
		return result, nil
	}
	if uploadErr := r.sandbox.UploadFiles(execCtx, execdURL, uploadFiles); uploadErr != nil {
		result.setError(fmt.Sprintf("uploading files to sandbox: %v", uploadErr))
		return result, nil
	}

	// Step 10: Run the skill command via ExecD.
	cmd := buildShellCommand(loadedSkill)
	timeoutMs := int(timeout.Milliseconds())

	cmdResult, runErr := r.sandbox.RunCommand(execCtx, execdURL, cmd, "/sandbox", timeoutMs)
	if runErr != nil {
		if execCtx.Err() != nil {
			result.Status = "timeout"
			result.setError(fmt.Sprintf("execution timed out after %s", timeout))
			return result, nil
		}
		result.setError(fmt.Sprintf("running command in sandbox: %v", runErr))
		return result, nil
	}

	// Collect logs from stdout/stderr.
	var logBuf strings.Builder
	if cmdResult.Stdout != "" {
		logBuf.WriteString(cmdResult.Stdout)
	}
	if cmdResult.Stderr != "" {
		if logBuf.Len() > 0 {
			logBuf.WriteString("\n")
		}
		logBuf.WriteString(cmdResult.Stderr)
	}
	result.Logs = truncateString(logBuf.String(), r.config.MaxOutputSize)

	// Step 11: Check for output.json.
	outputRC, dlErr := r.sandbox.DownloadFile(execCtx, execdURL, "/sandbox/out/output.json")
	if dlErr == nil {
		outputData, readErr := io.ReadAll(io.LimitReader(outputRC, 512<<20))
		outputRC.Close()
		if readErr != nil {
			log.Printf("runner: failed to read output.json for execution %s: %v", executionID, readErr)
		} else if json.Valid(outputData) {
			result.Output = json.RawMessage(outputData)
		} else {
			log.Printf("runner: output.json for execution %s is not valid JSON", executionID)
		}
	} else {
		// Log only if it is not a simple "file not found" (e.g. 404).
		if !strings.Contains(dlErr.Error(), "404") {
			log.Printf("runner: failed to download output.json for execution %s: %v", executionID, dlErr)
		}
	}

	// Step 12: Search for artifact files.
	if r.artifacts != nil {
		artifactFiles, searchErr := r.sandbox.SearchFiles(execCtx, execdURL, "/sandbox/out/files", "*")
		if searchErr != nil {
			log.Printf("runner: failed to search artifacts for %s: %v", executionID, searchErr)
		} else if len(artifactFiles) > 0 {
			// Download artifact files to a temp directory for the collector.
			tmpDir, collectErr := downloadArtifacts(execCtx, r.sandbox, execdURL, artifactFiles)
			if collectErr != nil {
				log.Printf("runner: failed to download artifacts for %s: %v", executionID, collectErr)
			} else {
				defer func() {
					if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
						log.Printf("runner: failed to remove artifact tmpdir %s: %v", tmpDir, removeErr)
					}
				}()

				artifactURL, filesList, uploadErr := r.artifacts.Collect(ctx, req.TenantID, executionID, tmpDir)
				if uploadErr != nil {
					log.Printf("runner: failed to collect artifacts for %s: %v", executionID, uploadErr)
				} else {
					result.FilesURL = artifactURL
					result.FilesList = filesList

					// Upload individual files so /v1/files/:id/download works.
					for _, fileName := range filesList {
						s3Key := fmt.Sprintf("%s/%s/%s", req.TenantID, executionID, fileName)
						filePath := filepath.Join(tmpDir, fileName)
						f, openErr := os.Open(filePath)
						if openErr != nil {
							log.Printf("runner: failed to open %s for individual upload: %v", fileName, openErr)
							continue
						}
						info, statErr := f.Stat()
						if statErr != nil {
							f.Close()
							continue
						}
						if _, upErr := r.artifacts.UploadObject(ctx, s3Key, f, info.Size(), detectRunnerContentType(fileName)); upErr != nil {
							log.Printf("runner: failed to upload individual file %s: %v", fileName, upErr)
						}
						f.Close()
					}

					// Create file records in the database for each collected artifact.
					for _, fileName := range filesList {
						s3Key := fmt.Sprintf("%s/%s/%s", req.TenantID, executionID, fileName)
						fileRecord := &store.File{
							TenantID:    req.TenantID,
							ExecutionID: executionID,
							Name:        fileName,
							ContentType: detectRunnerContentType(fileName),
							S3Key:       s3Key,
							Version:     1,
						}
						if _, createErr := r.store.CreateFile(ctx, fileRecord); createErr != nil {
							log.Printf("runner: failed to create file record for %s: %v", fileName, createErr)
						}
					}
				}
			}
		}
	}

	// Determine final status based on exit code.
	if cmdResult.ExitCode == 0 {
		result.Status = "success"
	} else {
		result.Status = "failed"
		if result.Error == nil {
			msg := fmt.Sprintf("command exited with code %d", cmdResult.ExitCode)
			if cmdResult.Error != "" {
				msg = cmdResult.Error
			}
			result.setError(msg)
		}
	}

	return result, nil
}

// pollExecD polls the ExecD health endpoint at the given interval until it
// responds successfully or the overall timeout is reached.
func pollExecD(ctx context.Context, client *sandbox.Client, execdURL string, interval, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Try immediately first.
	if err := client.Ping(ctx, execdURL); err == nil {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for execd: %w", ctx.Err())
		case <-deadline:
			return fmt.Errorf("execd did not become ready within %s", timeout)
		case <-ticker.C:
			if err := client.Ping(ctx, execdURL); err == nil {
				return nil
			}
		}
	}
}

// buildUploadFiles walks the extracted skill directory and builds the list
// of files to upload to the sandbox via ExecD. It places skill files under
// /sandbox/scripts/ and adds the input.json at /sandbox/input.json. It also
// creates the output directories via placeholder files.
func buildUploadFiles(skillDir string, inputJSON []byte) ([]sandbox.FileUpload, error) {
	var files []sandbox.FileUpload

	// Walk the skill directory and add all files under /sandbox/scripts/.
	if err := filepath.Walk(skillDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(skillDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		// Skip directories; ExecD creates intermediate directories on upload.
		if info.IsDir() {
			return nil
		}

		remotePath := "/sandbox/scripts/" + filepath.ToSlash(rel)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}

		// Preserve execute permission for script files.
		mode := int(info.Mode().Perm())
		if mode == 0 {
			mode = 0o644
		}

		files = append(files, sandbox.FileUpload{
			Path:    remotePath,
			Content: content,
			Mode:    mode,
		})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking skill directory: %w", err)
	}

	// Add input.json.
	files = append(files, sandbox.FileUpload{
		Path:    "/sandbox/input.json",
		Content: inputJSON,
		Mode:    0o644,
	})

	// Add placeholder files so that output directories exist.
	files = append(files, sandbox.FileUpload{
		Path:    "/sandbox/out/.keep",
		Content: []byte{},
		Mode:    0o644,
	})
	files = append(files, sandbox.FileUpload{
		Path:    "/sandbox/out/files/.keep",
		Content: []byte{},
		Mode:    0o644,
	})

	return files, nil
}

// downloadArtifacts downloads the listed artifact files from the sandbox
// into a local temporary directory and returns its path.
func downloadArtifacts(ctx context.Context, client *sandbox.Client, execdURL string, entries []sandbox.FileInfo) (string, error) {
	tmpDir, err := os.MkdirTemp("", "skillbox-artifacts-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir for artifacts: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	for _, entry := range entries {
		// Compute a safe relative path under the temp directory.
		// The entry.Path is the full path inside the sandbox, e.g.
		// /sandbox/out/files/report.pdf. We take just the filename
		// portion relative to the search directory.
		rel := filepath.Base(entry.Path)
		if strings.Contains(entry.Path, "/sandbox/out/files/") {
			rel = strings.TrimPrefix(entry.Path, "/sandbox/out/files/")
		}

		// Skip placeholder files.
		if rel == ".keep" {
			continue
		}

		// Guard against path traversal.
		if strings.Contains(rel, "..") {
			continue
		}

		localPath := filepath.Join(tmpDir, filepath.FromSlash(rel))

		// Ensure parent directory exists.
		if mkErr := os.MkdirAll(filepath.Dir(localPath), 0o750); mkErr != nil {
			return "", fmt.Errorf("creating dir for %s: %w", rel, mkErr)
		}

		rc, dlErr := client.DownloadFile(ctx, execdURL, entry.Path)
		if dlErr != nil {
			return "", fmt.Errorf("downloading artifact %s: %w", entry.Path, dlErr)
		}

		data, readErr := io.ReadAll(io.LimitReader(rc, 512<<20)) // 512 MiB per-file limit
		rc.Close()
		if readErr != nil {
			return "", fmt.Errorf("reading artifact %s: %w", entry.Path, readErr)
		}

		if writeErr := os.WriteFile(localPath, data, 0o644); writeErr != nil {
			return "", fmt.Errorf("writing artifact %s: %w", localPath, writeErr)
		}
	}

	success = true
	return tmpDir, nil
}

// buildShellCommand constructs the shell command string to run inside the
// sandbox based on the skill's language and whether dependency files are present.
func buildShellCommand(loaded *registry.LoadedSkill) string {
	entrypoint := "/sandbox/scripts/" + loaded.Entrypoint
	lang := loaded.Skill.Lang

	switch lang {
	case "python":
		if loaded.HasRequirements {
			return fmt.Sprintf(
				"pip install --no-cache-dir -r /sandbox/scripts/requirements.txt -t /tmp/deps && PYTHONPATH=/tmp/deps python %s",
				entrypoint,
			)
		}
		return fmt.Sprintf("python %s", entrypoint)

	case "node", "nodejs", "javascript":
		return fmt.Sprintf("node %s", entrypoint)

	case "bash":
		return fmt.Sprintf("bash %s", entrypoint)

	case "shell", "sh":
		return fmt.Sprintf("sh %s", entrypoint)

	default:
		return entrypoint
	}
}

// blockedEnvVars lists environment variable names that callers may not
// override. These are either security-sensitive (e.g. LD_PRELOAD) or
// reserved by the sandbox runtime (SANDBOX_*, SKILL_*).
var blockedEnvVars = map[string]bool{
	"PATH":            true,
	"HOME":            true,
	"LD_PRELOAD":      true,
	"LD_LIBRARY_PATH": true,
	"PYTHONPATH":      true,
	"NODE_PATH":       true,
	"NODE_OPTIONS":    true,
}

// isBlockedEnvVar returns true if the given key must not be set by callers.
func isBlockedEnvVar(key string) bool {
	if blockedEnvVars[key] {
		return true
	}
	upper := strings.ToUpper(key)
	return strings.HasPrefix(upper, "SANDBOX_") || strings.HasPrefix(upper, "SKILL_")
}

// shortID returns the first 12 characters of an ID for log output.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// truncateString truncates s to at most maxBytes bytes.
func truncateString(s string, maxBytes int64) string {
	if int64(len(s)) <= maxBytes {
		return s
	}
	return s[:maxBytes]
}

// detectRunnerContentType returns a MIME type based on the file extension.
func detectRunnerContentType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".csv"):
		return "text/csv"
	case strings.HasSuffix(lower, ".txt"), strings.HasSuffix(lower, ".log"):
		return "text/plain"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	case strings.HasSuffix(lower, ".xml"):
		return "application/xml"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".zip"):
		return "application/zip"
	case strings.HasSuffix(lower, ".tar"):
		return "application/x-tar"
	case strings.HasSuffix(lower, ".gz"), strings.HasSuffix(lower, ".tgz"):
		return "application/gzip"
	case strings.HasSuffix(lower, ".py"):
		return "text/x-python"
	case strings.HasSuffix(lower, ".js"):
		return "application/javascript"
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"):
		return "application/x-yaml"
	case strings.HasSuffix(lower, ".md"):
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}
