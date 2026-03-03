package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// ExecDPort is the standard port number for the ExecD agent inside sandboxes.
const ExecDPort = 44772

// Client communicates with the OpenSandbox lifecycle API (sandbox CRUD)
// and the ExecD API (in-sandbox file and command operations).
type Client struct {
	httpClient   *http.Client
	lifecycleURL string
	apiKey       string
}

// New creates a Client. lifecycleURL is the base (e.g. "http://opensandbox:8080/v1").
func New(lifecycleURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		httpClient:   httpClient,
		lifecycleURL: strings.TrimRight(lifecycleURL, "/"),
		apiKey:       apiKey,
	}
}

// SandboxOpts configures a new sandbox.
type SandboxOpts struct {
	Image          string
	Entrypoint     []string
	Env            map[string]string
	Metadata       map[string]string
	Timeout        int               // seconds (60-86400)
	ResourceLimits map[string]string // e.g. {"cpu":"500m","memory":"256Mi"}
	NetworkPolicy  *NetworkPolicy
}

// NetworkPolicy controls the sandbox's network access.
type NetworkPolicy struct {
	DefaultAction string       `json:"defaultAction"`
	Egress        []EgressRule `json:"egress,omitempty"`
}

// EgressRule describes a single egress permission.
type EgressRule struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

// SandboxResponse is returned after sandbox creation or retrieval.
type SandboxResponse struct {
	ID, State string
	ExpiresAt time.Time
	CreatedAt time.Time
	Metadata  map[string]string
}

// Endpoint describes a reachable port inside a running sandbox.
type Endpoint struct {
	Host    string
	Port    int
	URL     string
	Headers map[string]string
}

// FileUpload describes a single file to be uploaded into a sandbox.
type FileUpload struct {
	Path    string
	Content []byte
	Mode    int
}

// FileInfo describes a file found by SearchFiles.
type FileInfo struct {
	Path       string
	Size       int64
	ModifiedAt time.Time
}

// CommandResult holds the outcome of an in-sandbox command execution.
type CommandResult struct {
	Stdout, Stderr string
	ExitCode       int
	Error          string
	Duration       time.Duration
}

// CreateSandbox requests a new sandbox (HTTP 202, Pending state).
func (c *Client) CreateSandbox(ctx context.Context, opts SandboxOpts) (*SandboxResponse, error) {
	body := sandboxBody{
		Image:      imageURI{URI: opts.Image},
		Timeout:    opts.Timeout,
		Resources:  opts.ResourceLimits,
		Entrypoint: opts.Entrypoint,
		Env:        opts.Env,
		Metadata:   opts.Metadata,
		NetPolicy:  opts.NetworkPolicy,
	}
	var raw sandboxWire
	if err := c.lcPost(ctx, "/sandboxes", body, http.StatusAccepted, &raw); err != nil {
		return nil, err
	}
	return raw.decode()
}

// GetSandbox retrieves the current state of a single sandbox.
func (c *Client) GetSandbox(ctx context.Context, id string) (*SandboxResponse, error) {
	var raw sandboxWire
	if err := c.lcGet(ctx, "/sandboxes/"+url.PathEscape(id), nil, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw.decode()
}

// ListSandboxes returns sandboxes matching the given metadata filters.
func (c *Client) ListSandboxes(ctx context.Context, metadata map[string]string) ([]SandboxResponse, error) {
	params := url.Values{}
	for k, v := range metadata {
		params.Add("metadata", k+"="+v)
	}
	var rawList []sandboxWire
	if err := c.lcGet(ctx, "/sandboxes", params, http.StatusOK, &rawList); err != nil {
		return nil, err
	}
	out := make([]SandboxResponse, 0, len(rawList))
	for _, raw := range rawList {
		info, err := raw.decode()
		if err != nil {
			return nil, err
		}
		out = append(out, *info)
	}
	return out, nil
}

// DeleteSandbox terminates and removes a sandbox (HTTP 204).
func (c *Client) DeleteSandbox(ctx context.Context, id string) error {
	return c.lcDo(ctx, http.MethodDelete, "/sandboxes/"+url.PathEscape(id), nil, nil, http.StatusNoContent, nil)
}

// GetEndpoint discovers the externally reachable address for a sandbox port.
func (c *Client) GetEndpoint(ctx context.Context, sandboxID string, port int) (*Endpoint, error) {
	path := fmt.Sprintf("/sandboxes/%s/endpoints/%d", url.PathEscape(sandboxID), port)
	var raw endpointWire
	if err := c.lcGet(ctx, path, nil, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	u := raw.URL
	if u == "" {
		u = raw.Endpoint
	}
	if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	return &Endpoint{Host: raw.Host, Port: raw.Port, URL: u, Headers: raw.Headers}, nil
}

// DiscoverExecD calls GetEndpoint for the standard ExecD port (44772).
func (c *Client) DiscoverExecD(ctx context.Context, sandboxID string) (string, map[string]string, error) {
	ep, err := c.GetEndpoint(ctx, sandboxID, ExecDPort)
	if err != nil {
		return "", nil, err
	}
	return ep.URL, ep.Headers, nil
}

// WaitReady polls GetSandbox until "Running" or the context expires.
func (c *Client) WaitReady(ctx context.Context, id string) (*SandboxResponse, error) {
	delay := 250 * time.Millisecond
	for {
		info, err := c.GetSandbox(ctx, id)
		if err != nil {
			return nil, err
		}
		if info.State == "Running" {
			return info, nil
		}
		if info.State == "Failed" || info.State == "Terminated" {
			return info, fmt.Errorf("opensandbox: sandbox %s reached terminal state %q", id, info.State)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("opensandbox: waiting for sandbox %s: %w", id, ctx.Err())
		case <-time.After(delay):
		}
		if delay < 2*time.Second {
			delay *= 2
		}
	}
}

// Ping performs a health check against the ExecD instance.
func (c *Client) Ping(ctx context.Context, execdURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimURL(execdURL)+"/ping", nil)
	if err != nil {
		return fmt.Errorf("opensandbox: building ping request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensandbox: ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.errStatus("ping", resp)
	}
	return nil
}

// UploadFiles uploads files via ExecD's multipart endpoint (metadata+file pairs).
func (c *Client) UploadFiles(ctx context.Context, execdURL string, files []FileUpload) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		metaJSON, _ := json.Marshal(fileMetaWire{Path: f.Path, Mode: f.Mode})
		p, err := mw.CreateFormFile("metadata", "metadata.json")
		if err != nil {
			return fmt.Errorf("opensandbox: creating metadata part: %w", err)
		}
		if _, wErr := p.Write(metaJSON); wErr != nil {
			return fmt.Errorf("opensandbox: writing metadata: %w", wErr)
		}
		fp, err := mw.CreateFormFile("file", filepath.Base(f.Path))
		if err != nil {
			return fmt.Errorf("opensandbox: creating file part: %w", err)
		}
		if _, wErr := fp.Write(f.Content); wErr != nil {
			return fmt.Errorf("opensandbox: writing file content: %w", wErr)
		}
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("opensandbox: closing multipart writer: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimURL(execdURL)+"/files/upload", &buf)
	if err != nil {
		return fmt.Errorf("opensandbox: building upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensandbox: upload files: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return c.errStatus("upload files", resp)
	}
	return nil
}

// RunCommand executes a command inside the sandbox. The SSE response uses
// non-standard framing: raw JSON + "\n\n", optionally "data:"-prefixed.
func (c *Client) RunCommand(ctx context.Context, execdURL, cmd, cwd string, timeout int) (*CommandResult, error) {
	payload, _ := json.Marshal(cmdReqWire{Command: cmd, Cwd: cwd, Background: false, Timeout: timeout})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimURL(execdURL)+"/command", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("opensandbox: building command request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: run command: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.errStatus("run command", resp)
	}
	return parseSSEStream(resp.Body)
}

// DownloadFile retrieves a file from the sandbox. Caller must close the reader.
func (c *Client) DownloadFile(ctx context.Context, execdURL, path string) (io.ReadCloser, error) {
	u := trimURL(execdURL) + "/files/download?" + url.Values{"path": {path}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: building download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: download file %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, c.errStatus("download file", resp)
	}
	return resp.Body, nil
}

// SearchFiles lists files in the sandbox matching a glob pattern.
func (c *Client) SearchFiles(ctx context.Context, execdURL, dir, pattern string) ([]FileInfo, error) {
	u := trimURL(execdURL) + "/files/search?" + url.Values{"path": {dir}, "pattern": {pattern}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: building search request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: search files: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.errStatus("search files", resp)
	}
	var raw []fileInfoWire
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("opensandbox: decoding search results: %w", err)
	}
	out := make([]FileInfo, len(raw))
	for i, rf := range raw {
		out[i] = FileInfo{Path: rf.Path, Size: rf.Size, ModifiedAt: rf.ModifiedAt}
	}
	return out, nil
}

// lcDo executes a lifecycle API request. For POST, body is JSON-marshalled;
// for GET body should be nil. Query params are appended if non-empty.
func (c *Client) lcDo(ctx context.Context, method, path string, params url.Values, body any, expect int, dest any) error {
	endpoint := c.lifecycleURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("opensandbox: marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("opensandbox: building %s %s: %w", method, path, err)
	}
	req.Header.Set("OPEN-SANDBOX-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensandbox: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expect {
		return c.errStatus(method+" "+path, resp)
	}
	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("opensandbox: decoding response for %s %s: %w", method, path, err)
		}
	}
	return nil
}

func (c *Client) lcPost(ctx context.Context, path string, body any, expect int, dest any) error {
	return c.lcDo(ctx, http.MethodPost, path, nil, body, expect, dest)
}

func (c *Client) lcGet(ctx context.Context, path string, params url.Values, expect int, dest any) error {
	return c.lcDo(ctx, http.MethodGet, path, params, nil, expect, dest)
}

func (c *Client) errStatus(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("opensandbox: %s: unexpected status %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
}

func trimURL(u string) string { return strings.TrimRight(u, "/") }

// parseSSEStream reads the non-standard SSE stream from ExecD's /command
// endpoint. Events are bare JSON or "data: {json}", separated by "\n\n".
func parseSSEStream(r io.Reader) (*CommandResult, error) {
	result := &CommandResult{}
	var stdoutBuf, stderrBuf strings.Builder
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lineBuf strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if lineBuf.Len() > 0 {
				applySSE(lineBuf.String(), result, &stdoutBuf, &stderrBuf)
				lineBuf.Reset()
			}
			continue
		}
		if lineBuf.Len() > 0 {
			lineBuf.WriteString("\n")
		}
		lineBuf.WriteString(line)
	}
	if lineBuf.Len() > 0 {
		applySSE(lineBuf.String(), result, &stdoutBuf, &stderrBuf)
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("opensandbox: reading command stream: %w", err)
	}
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	return result, nil
}

func applySSE(raw string, result *CommandResult, stdout, stderr *strings.Builder) {
	data := strings.TrimSpace(raw)
	if strings.HasPrefix(data, "data:") {
		data = strings.TrimSpace(data[5:])
	}
	if data == "" {
		return
	}
	var ev sseEventWire
	if json.Unmarshal([]byte(data), &ev) != nil {
		return
	}
	switch ev.Type {
	case "stdout":
		stdout.WriteString(ev.Data)
	case "stderr":
		stderr.WriteString(ev.Data)
	case "error":
		result.Error = ev.Data
	case "execution_complete":
		result.ExitCode = ev.ExitCode
		if ev.DurationMs > 0 {
			result.Duration = time.Duration(ev.DurationMs) * time.Millisecond
		}
	}
}

type imageURI struct{ URI string `json:"uri"` }

type sandboxBody struct {
	Image      imageURI          `json:"image"`
	Timeout    int               `json:"timeout"`
	Resources  map[string]string `json:"resource_limits,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	NetPolicy  *NetworkPolicy    `json:"network_policy,omitempty"`
}

type sandboxWire struct {
	ID        string                             `json:"id"`
	Status    struct{ State string `json:"state"` } `json:"status"`
	ExpiresAt string                             `json:"expires_at"`
	CreatedAt string                             `json:"created_at"`
	Metadata  map[string]string                  `json:"metadata,omitempty"`
}

func (r *sandboxWire) decode() (*SandboxResponse, error) {
	exp, err := parseTime(r.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: parsing expires_at %q: %w", r.ExpiresAt, err)
	}
	cre, err := parseTime(r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("opensandbox: parsing created_at %q: %w", r.CreatedAt, err)
	}
	return &SandboxResponse{ID: r.ID, State: r.Status.State, ExpiresAt: exp, CreatedAt: cre, Metadata: r.Metadata}, nil
}

type endpointWire struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	URL      string            `json:"url"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
}

type (
	fileMetaWire struct {
		Path string `json:"path"`
		Mode int    `json:"mode"`
	}
	cmdReqWire struct {
		Command    string `json:"command"`
		Cwd        string `json:"cwd"`
		Background bool   `json:"background"`
		Timeout    int    `json:"timeout"`
	}
	sseEventWire struct {
		Type       string `json:"type"`
		Data       string `json:"data"`
		ExitCode   int    `json:"exitCode"`
		DurationMs int64  `json:"durationMs"`
	}
	fileInfoWire struct {
		Path       string    `json:"path"`
		Size       int64     `json:"size"`
		ModifiedAt time.Time `json:"modified_at"`
	}
)

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, f := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}
