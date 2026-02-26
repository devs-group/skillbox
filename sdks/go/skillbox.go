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

// Skill describes a registered skill definition.
type Skill struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer f.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write the multipart body in a goroutine so we can stream it into
	// the request without buffering the entire file in memory.
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
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
	defer resp.Body.Close()

	// Check the writer goroutine for errors.
	if writeErr := <-errCh; writeErr != nil {
		return fmt.Errorf("skillbox: write multipart body: %w", writeErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseAPIError(resp)
	}
	return nil
}

// ListSkills returns all skills registered on the server.
func (c *Client) ListSkills(ctx context.Context) ([]Skill, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/skills", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var skills []Skill
	if err := c.decodeResponse(resp, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// Health checks whether the Skillbox server is reachable. It returns nil
// on success or an error describing the failure.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("skillbox: download files: HTTP %d", resp.StatusCode)
	}

	return extractTarGz(resp.Body, destDir)
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
	defer gz.Close()

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
		target := filepath.Join(absDestDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), absDestDir+string(os.PathSeparator)) &&
			filepath.Clean(target) != absDestDir {
			return fmt.Errorf("skillbox: path traversal detected in tar entry: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("skillbox: create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("skillbox: create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("skillbox: write file %s: %w", path, err)
	}
	return nil
}
