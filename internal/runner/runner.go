package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
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

// Runner orchestrates skill execution in sandboxed Docker containers.
type Runner struct {
	docker    *client.Client
	config    *config.Config
	registry  *registry.Registry
	store     *store.Store
	artifacts *artifacts.Collector
	sem       chan struct{} // concurrency limiter
}

// New creates a Runner with all required dependencies.
func New(cfg *config.Config, docker *client.Client, reg *registry.Registry, st *store.Store, art *artifacts.Collector) *Runner {
	return &Runner{
		docker:    docker,
		config:    cfg,
		registry:  reg,
		store:     st,
		artifacts: art,
		sem:       make(chan struct{}, cfg.MaxConcurrentExecs),
	}
}

// Run executes a skill in a sandboxed Docker container. It handles the
// complete lifecycle: record creation, skill loading, container setup,
// execution, output collection, artifact uploading, and cleanup.
//
// The context controls the overall execution timeout. If the context is
// cancelled or times out, the container is killed and the execution is
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
	// Use the skill's DefaultImage() method which returns the custom image
	// if set, or resolves the default for the language.
	image := loadedSkill.Skill.DefaultImage()
	if err := ValidateImage(image, r.config.ImageAllowlist); err != nil {
		result.setError(fmt.Sprintf("image validation: %v", err))
		return result, nil
	}

	// Determine resource limits. Use skill-level overrides or server defaults.
	memoryBytes := r.config.DefaultMemory
	if loadedSkill.Skill.Resources.Memory != "" {
		parsed, parseErr := ParseMemoryLimit(loadedSkill.Skill.Resources.Memory)
		if parseErr != nil {
			result.setError(fmt.Sprintf("parsing memory limit: %v", parseErr))
			return result, nil
		}
		memoryBytes = parsed
	}

	cpuQuota := int64(r.config.DefaultCPU * 100000)
	if loadedSkill.Skill.Resources.CPU != "" {
		parsed, parseErr := ParseCPULimit(loadedSkill.Skill.Resources.CPU)
		if parseErr != nil {
			result.setError(fmt.Sprintf("parsing CPU limit: %v", parseErr))
			return result, nil
		}
		cpuQuota = parsed
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

	// Step 5: Build the container command based on language.
	cmd := buildCommand(loadedSkill)

	// Build environment variables.
	envVars := []string{
		"SANDBOX_INPUT=" + string(inputJSON),
		"SANDBOX_OUTPUT=/sandbox/out/output.json",
		"SANDBOX_FILES_DIR=/sandbox/out/files/",
		"SKILL_INSTRUCTIONS=" + loadedSkill.Skill.Instructions,
		"HOME=/tmp",
	}
	for k, v := range req.Env {
		if isBlockedEnvVar(k) {
			result.setError(fmt.Sprintf("env var %q is not allowed", k))
			return result, nil
		}
		envVars = append(envVars, k+"="+v)
	}

	// Step 6: Create container with full security hardening.
	// We use CopyToContainer to inject files instead of bind mounts, because
	// the skillbox server may run inside a container itself (Docker Compose /
	// K8s sidecar pattern) and host-path bind mounts would reference paths
	// on the Docker host rather than inside the server container.
	pidsLimit := int64(128)
	containerCfg := &container.Config{
		Image:      image,
		Cmd:        cmd,
		User:       "65534:65534",
		Env:        envVars,
		WorkingDir: "/sandbox",
		Labels: map[string]string{
			"managed-by": "skillbox",
			"tenant":     req.TenantID,
			"skill":      req.Skill,
			"execution":  executionID,
		},
	}
	hostCfg := &container.HostConfig{
		NetworkMode: "bridge",
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges:true"},
		Resources: container.Resources{
			Memory:     memoryBytes,
			MemorySwap: memoryBytes,
			CPUQuota:   cpuQuota,
			CPUPeriod:  100000,
			PidsLimit:  &pidsLimit,
		},
		Tmpfs: map[string]string{
			"/tmp": "rw,exec,nosuid,size=256m",
		},
		AutoRemove: false,
	}

	createResp, createErr := r.docker.ContainerCreate(execCtx, containerCfg, hostCfg, nil, nil, "")
	if createErr != nil {
		result.setError(fmt.Sprintf("creating container: %v", createErr))
		return result, nil
	}
	containerID := createResp.ID

	// Ensure container is always force-removed.
	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer removeCancel()
		if removeErr := r.docker.ContainerRemove(removeCtx, containerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); removeErr != nil {
			log.Printf("runner: failed to remove container %s: %v", shortID(containerID), removeErr)
		}
	}()

	// Step 6b: Build a single tar archive containing the entire /sandbox
	// tree (scripts, input.json, output dirs) and copy it to "/" in one call.
	sandboxTar, tarErr := buildSandboxTar(loadedSkill.Dir, inputJSON)
	if tarErr != nil {
		result.setError(fmt.Sprintf("creating sandbox tar: %v", tarErr))
		return result, nil
	}
	if cpErr := r.docker.CopyToContainer(execCtx, containerID, "/", sandboxTar, container.CopyToContainerOptions{}); cpErr != nil {
		result.setError(fmt.Sprintf("copying sandbox to container: %v", cpErr))
		return result, nil
	}

	// Step 7: Start container.
	if startErr := r.docker.ContainerStart(execCtx, containerID, container.StartOptions{}); startErr != nil {
		result.setError(fmt.Sprintf("starting container: %v", startErr))
		return result, nil
	}

	// Step 8: Wait for container to exit.
	waitCh, errCh := r.docker.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case waitResult := <-waitCh:
		exitCode = waitResult.StatusCode
		if waitResult.Error != nil {
			result.setError(waitResult.Error.Message)
		}
	case waitErr := <-errCh:
		if execCtx.Err() != nil {
			killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer killCancel()
			if killErr := r.docker.ContainerKill(killCtx, containerID, "SIGKILL"); killErr != nil {
				log.Printf("runner: failed to kill timed-out container %s: %v", shortID(containerID), killErr)
			}
			result.Status = "timeout"
			result.setError(fmt.Sprintf("execution timed out after %s", timeout))
			result.Logs = collectLogs(r.docker, containerID, r.config.MaxOutputSize)
			return result, nil
		}
		result.setError(fmt.Sprintf("waiting for container: %v", waitErr))
		return result, nil
	}

	// Step 10: Collect logs.
	result.Logs = collectLogs(r.docker, containerID, r.config.MaxOutputSize)

	// Step 11: Copy /sandbox/out/ back from the container and read output.json.
	outDir, outErr := r.copyFromContainer(execCtx, containerID, "/sandbox/out")
	if outErr != nil {
		log.Printf("runner: failed to copy output from container %s: %v", shortID(containerID), outErr)
	} else {
		defer func() {
			if removeErr := os.RemoveAll(outDir); removeErr != nil {
				log.Printf("runner: failed to remove outdir %s: %v", outDir, removeErr)
			}
		}()

		// Read output.json if it exists.
		outputPath := filepath.Join(outDir, "out", "output.json")
		if outputData, readErr := os.ReadFile(outputPath); readErr == nil {
			if json.Valid(outputData) {
				result.Output = json.RawMessage(outputData)
			} else {
				log.Printf("runner: output.json for execution %s is not valid JSON", executionID)
			}
		}

		// Step 12: Collect file artifacts.
		if r.artifacts != nil {
			filesDir := filepath.Join(outDir, "out", "files")
			artifactURL, filesList, collectErr := r.artifacts.Collect(ctx, req.TenantID, executionID, filesDir)
			if collectErr != nil {
				log.Printf("runner: failed to collect artifacts for %s: %v", executionID, collectErr)
			} else {
				result.FilesURL = artifactURL
				result.FilesList = filesList
			}
		}
	}

	// Determine final status based on exit code.
	if exitCode == 0 {
		result.Status = "success"
	} else {
		result.Status = "failed"
		if result.Error == nil {
			result.setError(fmt.Sprintf("container exited with code %d", exitCode))
		}
	}

	return result, nil
}

// buildSandboxTar creates a single tar archive containing the entire
// /sandbox directory tree that will be extracted at "/" in the container:
//
//	sandbox/                     (dir)
//	sandbox/scripts/...          (skill files from skillDir)
//	sandbox/input.json           (input data)
//	sandbox/out/                 (dir, writable)
//	sandbox/out/files/           (dir, writable)
//	sandbox/out/output.json      (will be created by the skill)
func buildSandboxTar(skillDir string, inputJSON []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Helper to add a directory entry.
	addDir := func(name string, mode int64) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeDir,
			Mode:     mode,
		})
	}

	// Create the directory structure.
	for _, d := range []string{"sandbox/", "sandbox/scripts/", "sandbox/out/", "sandbox/out/files/"} {
		if err := addDir(d, 0o777); err != nil {
			return nil, fmt.Errorf("adding dir %s: %w", d, err)
		}
	}

	// Walk the skill directory and add all files under sandbox/scripts/.
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

		tarName := "sandbox/scripts/" + filepath.ToSlash(rel)
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Name:     tarName + "/",
				Typeflag: tar.TypeDir,
				Mode:     0o755,
			})
		}

		header, headerErr := tar.FileInfoHeader(info, "")
		if headerErr != nil {
			return headerErr
		}
		header.Name = tarName

		if writeErr := tw.WriteHeader(header); writeErr != nil {
			return writeErr
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer f.Close()
		_, cpErr := io.Copy(tw, f)
		return cpErr
	}); err != nil {
		return nil, fmt.Errorf("adding skill files: %w", err)
	}

	// Add input.json.
	if err := tw.WriteHeader(&tar.Header{
		Name: "sandbox/input.json",
		Mode: 0o644,
		Size: int64(len(inputJSON)),
	}); err != nil {
		return nil, fmt.Errorf("adding input.json header: %w", err)
	}
	if _, err := tw.Write(inputJSON); err != nil {
		return nil, fmt.Errorf("writing input.json: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// copyFromContainer copies a path from the container to a temporary directory
// on the local filesystem and returns the temp directory path.
func (r *Runner) copyFromContainer(ctx context.Context, containerID, containerPath string) (string, error) {
	reader, _, err := r.docker.CopyFromContainer(ctx, containerID, containerPath)
	if err != nil {
		return "", fmt.Errorf("copy from container: %w", err)
	}
	defer reader.Close()

	tmpDir, err := os.MkdirTemp("", "skillbox-out-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	tr := tar.NewReader(reader)
	for {
		header, tarErr := tr.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			_ = os.RemoveAll(tmpDir)
			return "", fmt.Errorf("reading tar: %w", tarErr)
		}

		target := filepath.Join(tmpDir, filepath.FromSlash(header.Name))

		// Guard against path traversal.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(tmpDir)+string(filepath.Separator)) &&
			filepath.Clean(target) != filepath.Clean(tmpDir) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if mkErr := os.MkdirAll(target, 0o750); mkErr != nil {
				_ = os.RemoveAll(tmpDir)
				return "", mkErr
			}
		case tar.TypeReg:
			if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
				_ = os.RemoveAll(tmpDir)
				return "", mkErr
			}
			f, createErr := os.Create(target)
			if createErr != nil {
				_ = os.RemoveAll(tmpDir)
				return "", createErr
			}
			if _, cpErr := io.Copy(f, io.LimitReader(tr, 512<<20)); cpErr != nil { // 512 MiB per-file limit
				_ = f.Close()
				_ = os.RemoveAll(tmpDir)
				return "", cpErr
			}
			_ = f.Close()
		}
	}

	return tmpDir, nil
}

// buildCommand constructs the shell command to run inside the container
// based on the skill's language and whether dependency files are present.
func buildCommand(loaded *registry.LoadedSkill) []string {
	entrypoint := "/sandbox/scripts/" + loaded.Entrypoint
	lang := loaded.Skill.Lang

	switch lang {
	case "python":
		if loaded.HasRequirements {
			// Install dependencies to a temp directory, then run the script
			// with PYTHONPATH set so imports resolve correctly.
			return []string{
				"sh", "-c",
				fmt.Sprintf(
					"pip install --no-cache-dir -r /sandbox/scripts/requirements.txt -t /tmp/deps && PYTHONPATH=/tmp/deps python %s",
					entrypoint,
				),
			}
		}
		return []string{"python", entrypoint}

	case "node", "nodejs", "javascript":
		return []string{"node", entrypoint}

	case "bash":
		return []string{"bash", entrypoint}

	case "shell", "sh":
		return []string{"sh", entrypoint}

	default:
		// Fallback: try to run directly.
		return []string{entrypoint}
	}
}

// collectLogs reads stdout and stderr from a container and returns the
// combined output as a string, truncated to maxSize bytes.
func collectLogs(docker *client.Client, containerID string, maxSize int64) string {
	logCtx, logCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer logCancel()

	reader, err := docker.ContainerLogs(logCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
	})
	if err != nil {
		log.Printf("runner: failed to read logs for container %s: %v", shortID(containerID), err)
		return ""
	}
	defer reader.Close()

	var buf bytes.Buffer
	// Docker multiplexes stdout/stderr with an 8-byte header per frame.
	// We use io.Copy with a LimitReader to cap the total size.
	if _, cpErr := io.Copy(&buf, io.LimitReader(reader, maxSize)); cpErr != nil {
		log.Printf("runner: error reading logs for container %s: %v", shortID(containerID), cpErr)
	}

	return stripDockerLogHeaders(buf.Bytes())
}

// stripDockerLogHeaders removes Docker's 8-byte multiplexing headers from
// container log output. Each frame starts with [stream_type(1)][0(3)][size(4)]
// followed by the payload. If the data does not appear to have headers, it
// is returned as-is.
func stripDockerLogHeaders(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var cleaned strings.Builder
	cleaned.Grow(len(data))

	pos := 0
	for pos < len(data) {
		// Need at least 8 bytes for the header.
		if pos+8 > len(data) {
			// Write remaining bytes as-is.
			cleaned.Write(data[pos:])
			break
		}

		// Check if this looks like a Docker log header.
		// Stream types: 0=stdin, 1=stdout, 2=stderr.
		streamType := data[pos]
		if (streamType == 0 || streamType == 1 || streamType == 2) &&
			data[pos+1] == 0 && data[pos+2] == 0 && data[pos+3] == 0 {
			// Read the payload size (big-endian uint32).
			size := int(data[pos+4])<<24 | int(data[pos+5])<<16 | int(data[pos+6])<<8 | int(data[pos+7])
			pos += 8

			end := min(pos+size, len(data))
			cleaned.Write(data[pos:end])
			pos = end
		} else {
			// Not a Docker header; write byte and advance.
			cleaned.WriteByte(data[pos])
			pos++
		}
	}

	return cleaned.String()
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

// shortID returns the first 12 characters of a container ID for log output.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
