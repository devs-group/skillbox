// Package skillbox provides a Go client for the Skillbox API — an open-source
// secure skill execution runtime for AI agents.
//
// The client wraps the Skillbox REST API with zero dependencies beyond the
// Go standard library.
//
// # Quick start
//
//	client := skillbox.New("http://localhost:8080", "sk-your-api-key")
//	result, err := client.Run(ctx, skillbox.RunRequest{
//	    Skill: "data-analysis",
//	    Input: json.RawMessage(`{"data": [1, 2, 3]}`),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Status, string(result.Output))
//
// # Authentication
//
// Pass the API key directly to [New], or leave it empty to read from the
// SKILLBOX_API_KEY environment variable automatically.
//
// # Multi-tenancy
//
// Use [WithTenant] to scope all requests to a specific tenant:
//
//	client := skillbox.New(baseURL, apiKey, skillbox.WithTenant("tenant-42"))
package skillbox

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// --------------------------------------------------------------------
// Types
// --------------------------------------------------------------------

// Client communicates with the Skillbox API. Create one with [New] and
// reuse it for the lifetime of your application — it is safe for
// concurrent use.
type Client struct {
	baseURL    string
	apiKey     string
	tenantID   string
	httpClient *http.Client
}

// RunRequest describes a skill execution. Skill is the only required field.
type RunRequest struct {
	// Skill is the name of the registered skill to execute (e.g. "data-analysis").
	Skill string `json:"skill"`

	// Version pins a specific skill version. When empty the latest version is used.
	Version string `json:"version,omitempty"`

	// Input is the JSON payload forwarded to the skill's entrypoint.
	Input json.RawMessage `json:"input,omitempty"`

	// Env injects additional environment variables into the execution container.
	Env map[string]string `json:"env,omitempty"`

	// InputFiles lists file IDs (from POST /v1/files) to inject into the
	// sandbox at /sandbox/input/<filename> before execution.
	InputFiles []string `json:"input_files,omitempty"`

	// Entrypoint overrides the skill's default entrypoint script. When set,
	// the runner executes this file instead of the one detected in the archive
	// (e.g. "analysis.py" instead of "main.py"). The path is relative to the
	// skill's root directory.
	Entrypoint string `json:"entrypoint,omitempty"`

	// SessionID links this execution to a persistent session workspace. Files
	// written to /sandbox/out/session/ are preserved and re-mounted on the
	// next execution in the same session.
	SessionID string `json:"session_id,omitempty"`
}

// RunResult is the response returned after a skill execution completes.
type RunResult struct {
	// ExecutionID is the unique identifier for this execution.
	ExecutionID string `json:"execution_id"`

	// Status is the terminal state: "completed", "failed", "timeout", etc.
	Status string `json:"status"`

	// Output is the JSON payload produced by the skill.
	Output json.RawMessage `json:"output"`

	// FilesURL is a pre-signed URL to download a tar.gz archive of output
	// files. Empty when the execution produced no files.
	FilesURL string `json:"files_url"`

	// FilesList enumerates the relative paths inside the archive.
	FilesList []string `json:"files_list"`

	// Logs contains the combined stdout/stderr captured during execution.
	Logs string `json:"logs"`

	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// Error holds a human-readable message when Status indicates failure.
	Error string `json:"error"`
}

// HasFiles reports whether the execution produced downloadable output files.
func (r *RunResult) HasFiles() bool {
	return r.FilesURL != ""
}

// Skill describes a registered skill definition as returned by list endpoints.
type Skill struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Lang        string `json:"lang,omitempty"`
	Mode        string `json:"mode,omitempty"`
}

// SkillDetail is the full skill definition returned by GetSkill, including
// the SKILL.md instructions body. This is the key data structure for agents
// that need to understand what a skill does before executing it.
type SkillDetail struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Lang         string            `json:"lang"`
	Image        string            `json:"image,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	Resources    map[string]string `json:"resources,omitempty"`
	Mode         string            `json:"mode"`
}

// FileInfo represents a file record from the Skillbox API.
type FileInfo struct {
	ID          string  `json:"id"`
	TenantID    string  `json:"tenant_id"`
	SessionID   string  `json:"session_id,omitempty"`
	ExecutionID string  `json:"execution_id,omitempty"`
	Name        string  `json:"name"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	S3Key       string  `json:"s3_key"`
	Version     int     `json:"version"`
	ParentID    *string `json:"parent_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// FileFilter specifies query parameters for listing files.
type FileFilter struct {
	SessionID   string
	ExecutionID string
	Limit       int
	Offset      int
}

// SkillFileEntry represents a single source file extracted from a skill archive.
type SkillFileEntry struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SizeBytes int    `json:"size_bytes"`
}

// SandboxExecRequest describes a command to execute in a session sandbox.
type SandboxExecRequest struct {
	// Command is the bash command string to execute.
	Command string `json:"command"`

	// WorkDir sets the working directory for the command. Defaults to
	// /sandbox/session if empty.
	WorkDir string `json:"workdir,omitempty"`

	// TimeoutMs is the maximum execution time in milliseconds. Defaults to
	// 30000 (30 seconds) if zero.
	TimeoutMs int `json:"timeout_ms,omitempty"`
}

// SandboxExecResponse holds the result of a sandbox command execution.
type SandboxExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// SandboxDirEntry describes a single entry in a sandbox directory listing.
type SandboxDirEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ToolDefinition is an LLM-compatible tool schema. Parameters uses
// map[string]any to match the convention in most Go LLM libraries.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Option configures a [Client]. Pass options to [New].
type Option func(*Client)

// APIError is returned when the Skillbox API responds with a non-2xx status
// code and a structured error body.
type APIError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int `json:"-"`

	// ErrorCode is a machine-readable error identifier (e.g. "invalid_request").
	ErrorCode string `json:"error"`

	// Message is a human-readable description of what went wrong.
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("skillbox: %d %s: %s", e.StatusCode, e.ErrorCode, e.Message)
	}
	if e.ErrorCode != "" {
		return fmt.Sprintf("skillbox: %d %s", e.StatusCode, e.ErrorCode)
	}
	return fmt.Sprintf("skillbox: %d", e.StatusCode)
}

// --------------------------------------------------------------------
// Constructor & Options
// --------------------------------------------------------------------

// New creates a new Skillbox [Client].
//
// If apiKey is empty, New falls back to the SKILLBOX_API_KEY environment
// variable. When neither is set, requests are sent without authentication
// (useful for local development without auth enabled).
//
// The returned client uses [http.DefaultClient] unless overridden with
// [WithHTTPClient].
func New(baseURL, apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("SKILLBOX_API_KEY")
	}

	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithTenant sets the X-Tenant-ID header on every request, scoping all
// operations to the given tenant.
func WithTenant(tenantID string) Option {
	return func(c *Client) {
		c.tenantID = tenantID
	}
}

// WithHTTPClient replaces the default HTTP client. Use this to configure
// custom timeouts, transport settings, or instrumentation.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// --------------------------------------------------------------------
// Public API
// --------------------------------------------------------------------

// Run executes a skill and blocks until the execution completes. It
// returns the full [RunResult] including output, logs, and file metadata.
//
// The provided context controls the HTTP request lifetime. Use
// [context.WithTimeout] to enforce an upper bound on how long the caller
// is willing to wait.
func (c *Client) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: marshal run request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/v1/executions", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result RunResult
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExecution retrieves the current state of a previously started execution.
func (c *Client) GetExecution(ctx context.Context, id string) (*RunResult, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/executions/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result RunResult
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExecutionLogs returns the combined stdout/stderr logs for an execution.
func (c *Client) GetExecutionLogs(ctx context.Context, id string) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/executions/"+id+"/logs", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", c.parseAPIError(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("skillbox: read logs response: %w", err)
	}

	// Try JSON envelope first; fall back to raw text.
	var envelope struct {
		Logs string `json:"logs"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Logs != "" {
		return envelope.Logs, nil
	}
	return string(data), nil
}

// RegisterSkill uploads a skill zip archive to the Skillbox server.
// zipPath must point to a readable .zip file on disk.
func (c *Client) RegisterSkill(ctx context.Context, zipPath string) error {
	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("skillbox: open skill archive: %w", err)
	}
	defer f.Close() //nolint:errcheck

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write the multipart body in a goroutine so we can stream it into
	// the request without buffering the entire file in memory.
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close() //nolint:errcheck
		part, err := writer.CreateFormFile("file", filepath.Base(zipPath))
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			errCh <- err
			return
		}
		errCh <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/skills", pr)
	if err != nil {
		return fmt.Errorf("skillbox: create register request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("skillbox: register skill: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Check the writer goroutine for errors.
	if writeErr := <-errCh; writeErr != nil {
		return fmt.Errorf("skillbox: write multipart body: %w", writeErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// CreateFromFieldsRequest describes a skill to create from structured fields
// rather than a pre-built zip archive. The server generates SKILL.md and
// packages the zip internally. Upsert semantics: if the skill already exists,
// it is replaced.
type CreateFromFieldsRequest struct {
	// Name is the skill identifier (kebab-case, alphanumeric + hyphens).
	Name string `json:"name"`

	// Description is a short summary of what the skill does.
	Description string `json:"description"`

	// Lang is the runtime language: "python" (default), "node", or "bash".
	Lang string `json:"lang,omitempty"`

	// Code is the entrypoint script content.
	Code string `json:"code"`

	// Instructions is the SKILL.md body text placed after the frontmatter.
	Instructions string `json:"instructions,omitempty"`

	// Version is the semver string. Defaults to "1.0.0" if empty.
	Version string `json:"version,omitempty"`
}

// UpsertSkillFromFields creates or replaces a skill from structured fields.
// The server builds the SKILL.md and zip archive internally.
func (c *Client) UpsertSkillFromFields(ctx context.Context, req CreateFromFieldsRequest) (*Skill, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: marshal create request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/v1/skills/from-fields", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var skill Skill
	if err := c.decodeResponse(resp, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// ListSkills returns all skills registered on the server. The response
// includes descriptions so callers can decide which skill to use.
func (c *Client) ListSkills(ctx context.Context) ([]Skill, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/skills", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var skills []Skill
	if err := c.decodeResponse(resp, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// GetSkill retrieves the full metadata for a specific skill version,
// including the SKILL.md instructions body. Use this to understand what
// a skill does, what input it expects, and how it behaves before calling Run.
func (c *Client) GetSkill(ctx context.Context, name, version string) (*SkillDetail, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/skills/"+name+"/"+version, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var detail SkillDetail
	if err := c.decodeResponse(resp, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetSkillFiles retrieves the source files from a skill archive. Each entry
// includes the file path, content, and size in bytes. Use the optional path
// parameter to retrieve a single file.
func (c *Client) GetSkillFiles(ctx context.Context, name, version string) ([]SkillFileEntry, error) {
	path := "/v1/skills/" + name + "/" + version + "/files"
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var files []SkillFileEntry
	if err := c.decodeResponse(resp, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetSkillFile retrieves a single source file from a skill archive by path.
func (c *Client) GetSkillFile(ctx context.Context, name, version, filePath string) (*SkillFileEntry, error) {
	p := "/v1/skills/" + name + "/" + version + "/files?path=" + url.QueryEscape(filePath)
	resp, err := c.doRequest(ctx, http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var files []SkillFileEntry
	if err := c.decodeResponse(resp, &files); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, &APIError{StatusCode: 404, ErrorCode: "not_found", Message: "file not found: " + filePath}
	}
	return &files[0], nil
}

// DeleteSkill removes a specific skill version. The server responds with
// 204 No Content on success.
func (c *Client) DeleteSkill(ctx context.Context, name, version string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/v1/skills/"+name+"/"+version, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// Health checks whether the Skillbox server is reachable. It returns nil
// on success or an error describing the failure.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// DownloadFiles fetches the output file archive from a completed execution,
// decompresses the gzip layer, and extracts the tar entries into destDir.
//
// If the execution produced no files ([RunResult.HasFiles] returns false),
// DownloadFiles is a no-op and returns nil.
//
// All tar entry paths are validated to prevent path-traversal attacks —
// entries that would escape destDir cause an immediate error.
func (c *Client) DownloadFiles(ctx context.Context, result *RunResult, destDir string) error {
	if !result.HasFiles() {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.FilesURL, nil)
	if err != nil {
		return fmt.Errorf("skillbox: create download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("skillbox: download files: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("skillbox: download files: HTTP %d", resp.StatusCode)
	}

	return extractTarGz(resp.Body, destDir)
}

// --------------------------------------------------------------------
// File Management
// --------------------------------------------------------------------

// ListFiles returns files matching the given filter criteria. Use
// [FileFilter] to scope results by session, execution, or page through
// results with limit/offset.
func (c *Client) ListFiles(ctx context.Context, filter FileFilter) ([]FileInfo, error) {
	params := url.Values{}
	if filter.SessionID != "" {
		params.Set("session_id", filter.SessionID)
	}
	if filter.ExecutionID != "" {
		params.Set("execution_id", filter.ExecutionID)
	}
	if filter.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", filter.Offset))
	}

	path := "/v1/files"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var files []FileInfo
	if err := c.decodeResponse(resp, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetFile retrieves the metadata for a single file by its ID.
func (c *Client) GetFile(ctx context.Context, id string) (*FileInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/files/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var file FileInfo
	if err := c.decodeResponse(resp, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// DownloadFile fetches the raw content of a file and writes it to destPath
// on disk. The destination path is validated to prevent path-traversal
// attacks — it must resolve to an absolute path that does not contain "..".
func (c *Client) DownloadFile(ctx context.Context, id, destPath string) error {
	// Validate destination path to prevent path traversal.
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("skillbox: resolve destination path: %w", err)
	}
	if strings.Contains(absPath, "..") {
		return fmt.Errorf("skillbox: path traversal detected in destination: %s", destPath)
	}

	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/files/"+id+"/download", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}

	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return fmt.Errorf("skillbox: create parent directory: %w", err)
	}

	f, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- path is validated above
	if err != nil {
		return fmt.Errorf("skillbox: create destination file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("skillbox: write file content: %w", err)
	}
	return nil
}

// UpdateFile uploads a new version of an existing file. The file at
// filePath is sent as a multipart form with field name "file". The server
// responds with the updated [FileInfo] including the new version number.
func (c *Client) UpdateFile(ctx context.Context, id, filePath string) (*FileInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("skillbox: open file for update: %w", err)
	}
	defer f.Close() //nolint:errcheck

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close() //nolint:errcheck
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			errCh <- err
			return
		}
		errCh <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/v1/files/"+id, pr)
	if err != nil {
		return nil, fmt.Errorf("skillbox: create update request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: update file: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if writeErr := <-errCh; writeErr != nil {
		return nil, fmt.Errorf("skillbox: write multipart body: %w", writeErr)
	}

	var file FileInfo
	if err := c.decodeResponse(resp, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// DeleteFile removes a file by its ID. The server responds with 204 No
// Content on success.
func (c *Client) DeleteFile(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/v1/files/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// UploadFile uploads a new file to the Skillbox server. The file at filePath
// is sent as a multipart form with field name "file". The server responds
// with the created [FileInfo] including the assigned ID.
func (c *Client) UploadFile(ctx context.Context, filePath string) (*FileInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("skillbox: open file for upload: %w", err)
	}
	defer f.Close() //nolint:errcheck

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close() //nolint:errcheck
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			errCh <- err
			return
		}
		errCh <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/files", pr)
	if err != nil {
		return nil, fmt.Errorf("skillbox: create upload request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: upload file: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if writeErr := <-errCh; writeErr != nil {
		return nil, fmt.Errorf("skillbox: write multipart body: %w", writeErr)
	}

	var file FileInfo
	if err := c.decodeResponse(resp, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// UploadFileFromReader uploads a file from an io.Reader. This avoids writing
// to disk when the content is already in memory or streamed from another source.
func (c *Client) UploadFileFromReader(ctx context.Context, filename string, r io.Reader) (*FileInfo, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close() //nolint:errcheck
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, r); err != nil {
			errCh <- err
			return
		}
		errCh <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/files", pr)
	if err != nil {
		return nil, fmt.Errorf("skillbox: create upload request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: upload file: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if writeErr := <-errCh; writeErr != nil {
		return nil, fmt.Errorf("skillbox: write multipart body: %w", writeErr)
	}

	var file FileInfo
	if err := c.decodeResponse(resp, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// ListFileVersions returns all versions of a file, ordered by version
// number. Each entry is a full [FileInfo] record.
func (c *Client) ListFileVersions(ctx context.Context, id string) ([]FileInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/files/"+id+"/versions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var versions []FileInfo
	if err := c.decodeResponse(resp, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}

// --------------------------------------------------------------------
// Session Workspace
// --------------------------------------------------------------------

// ListSessionFiles returns all files in a session workspace.
func (c *Client) ListSessionFiles(ctx context.Context, sessionID string) ([]FileInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/sessions/"+sessionID+"/files", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var files []FileInfo
	if err := c.decodeResponse(resp, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetSessionFile retrieves the content of a specific file from a session
// workspace. The caller is responsible for closing the returned reader.
func (c *Client) GetSessionFile(ctx context.Context, sessionID, filename string) (io.ReadCloser, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/sessions/"+sessionID+"/files/"+filename, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close() //nolint:errcheck
		return nil, c.parseAPIError(resp)
	}
	return resp.Body, nil
}

// DeleteSessionFile removes a specific file from a session workspace.
func (c *Client) DeleteSessionFile(ctx context.Context, sessionID, filename string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/v1/sessions/"+sessionID+"/files/"+filename, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// DeleteSession removes a session and all its associated files.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// --------------------------------------------------------------------
// Sandbox Shell
// --------------------------------------------------------------------

// SandboxExecute runs a bash command inside the session's sandbox. The
// sandbox is created automatically on first use and persists for the
// session's lifetime.
func (c *Client) SandboxExecute(ctx context.Context, sessionID string, req SandboxExecRequest) (*SandboxExecResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: marshal sandbox exec request: %w", err)
	}

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/execute", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result SandboxExecResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SandboxReadFile reads the content of a file inside the session sandbox.
func (c *Client) SandboxReadFile(ctx context.Context, sessionID, path string) (string, error) {
	body, _ := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: path})

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/read-file", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Content string `json:"content"`
		Size    int64  `json:"size"`
	}
	if err := c.decodeResponse(resp, &result); err != nil {
		return "", err
	}
	return result.Content, nil
}

// SandboxDownloadFile downloads raw binary file content from the session sandbox.
// Unlike SandboxReadFile, this returns raw bytes without JSON string encoding,
// making it safe for binary files (PDFs, images, etc.).
func (c *Client) SandboxDownloadFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	body, _ := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: path})

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/download-file", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skillbox: download file: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("skillbox: download file read body: %w", err)
	}
	return data, nil
}

// SandboxWriteFile writes content to a file inside the session sandbox.
// When appendFlag is true, the content is appended to the existing file.
func (c *Client) SandboxWriteFile(ctx context.Context, sessionID, path, content string, appendFlag bool) error {
	body, _ := json.Marshal(struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Append  bool   `json:"append,omitempty"`
	}{Path: path, Content: content, Append: appendFlag})

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/write-file", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// MountSkillToSession uploads a skill's files directly into the session sandbox
// at /sandbox/scripts/, without creating a throwaway execution sandbox.
func (c *Client) MountSkillToSession(ctx context.Context, sessionID, skill, version string) error {
	body, _ := json.Marshal(struct {
		Skill   string `json:"skill"`
		Version string `json:"version"`
	}{Skill: skill, Version: version})

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/upload-skill", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// SandboxListDir lists directory contents inside the session sandbox.
// maxDepth controls recursion depth (0 uses the server default of 2).
func (c *Client) SandboxListDir(ctx context.Context, sessionID, path string, maxDepth int) ([]SandboxDirEntry, error) {
	body, _ := json.Marshal(struct {
		Path     string `json:"path"`
		MaxDepth int    `json:"max_depth,omitempty"`
	}{Path: path, MaxDepth: maxDepth})

	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/list-dir", sessionID, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Entries []SandboxDirEntry `json:"entries"`
	}
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// SandboxSync persists the session sandbox's workspace files to MinIO
// so they survive sandbox restarts.
func (c *Client) SandboxSync(ctx context.Context, sessionID string) error {
	resp, err := c.doSessionRequest(ctx, http.MethodPost, "/v1/sandbox/sync", sessionID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// SandboxDestroy tears down the session's sandbox. This does not delete
// the session or its persisted files — only the running sandbox instance.
func (c *Client) SandboxDestroy(ctx context.Context, sessionID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/v1/sandbox/"+sessionID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// --------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------

// doRequest builds and executes an HTTP request against the Skillbox API.
// It sets authentication, tenant, and content-type headers automatically.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("skillbox: create request: %w", err)
	}

	c.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: %s %s: %w", method, path, err)
	}

	return resp, nil
}

// doSessionRequest is like doRequest but additionally sets the X-Session-ID
// header required by sandbox shell endpoints.
func (c *Client) doSessionRequest(ctx context.Context, method, path, sessionID string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("skillbox: create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("X-Session-ID", sessionID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillbox: %s %s: %w", method, path, err)
	}

	return resp, nil
}

// setHeaders applies authentication and tenant headers to a request.
func (c *Client) setHeaders(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
}

// decodeResponse checks for a non-2xx status and decodes the JSON body
// into target. On error responses it returns a structured [*APIError].
func (c *Client) decodeResponse(resp *http.Response, target interface{}) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("skillbox: decode response: %w", err)
	}
	return nil
}

// parseAPIError reads the response body and returns a structured [*APIError].
func (c *Client) parseAPIError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiErr
	}

	// Attempt to parse the body as structured JSON. If that fails, treat
	// the raw body as the message.
	if json.Unmarshal(data, apiErr) != nil {
		apiErr.Message = strings.TrimSpace(string(data))
	}
	apiErr.StatusCode = resp.StatusCode

	return apiErr
}

// extractTarGz decompresses a gzip stream and extracts the contained tar
// archive into destDir. It validates every entry path to prevent directory
// traversal attacks.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("skillbox: decompress gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck

	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("skillbox: resolve destination: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("skillbox: read tar entry: %w", err)
		}

		// Resolve the target path and ensure it stays inside destDir.
		target := filepath.Join(absDestDir, header.Name) // #nosec G305 -- path traversal is checked below
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), absDestDir+string(os.PathSeparator)) &&
			filepath.Clean(target) != absDestDir {
			return fmt.Errorf("skillbox: path traversal detected in tar entry: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("skillbox: create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("skillbox: create parent directory for %s: %w", target, err)
			}
			if err := writeFile(target, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeFile creates a file at path with the given mode and copies content
// from r into it.
func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) // #nosec G304 -- path is validated by caller
	if err != nil {
		return fmt.Errorf("skillbox: create file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("skillbox: write file %s: %w", path, err)
	}
	return nil
}
