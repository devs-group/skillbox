package skillbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrUnknownTool is returned when Handle receives an unrecognized tool name.
var ErrUnknownTool = errors.New("unknown workspace tool")

// WorkspaceToolkit dispatches LLM tool calls to sandbox operations.
// Create one per session via [NewWorkspaceToolkit].
type WorkspaceToolkit struct {
	client      *Client
	sessionID   string
	bashTimeout time.Duration
}

// WorkspaceOption configures a [WorkspaceToolkit].
type WorkspaceOption func(*WorkspaceToolkit)

// WithBashTimeout sets the bash command timeout. Default: 120s.
func WithBashTimeout(d time.Duration) WorkspaceOption {
	return func(t *WorkspaceToolkit) { t.bashTimeout = d }
}

// NewWorkspaceToolkit creates a toolkit scoped to a session.
// The Client must be configured with [WithTenant] for multi-tenant use.
func NewWorkspaceToolkit(client *Client, sessionID string, opts ...WorkspaceOption) *WorkspaceToolkit {
	t := &WorkspaceToolkit{
		client:      client,
		sessionID:   sessionID,
		bashTimeout: 120 * time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// FileRef describes a file uploaded via present_files.
type FileRef struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// ToolDefinitions returns LLM tool definitions for workspace tools.
// Returns: bash, read_file, write_file, ls, present_files.
func (t *WorkspaceToolkit) ToolDefinitions() []ToolDefinition {
	return workspaceToolDefs
}

// Handle dispatches an LLM tool call to the appropriate sandbox operation.
// Returns output text for the LLM and optional file references (present_files only).
func (t *WorkspaceToolkit) Handle(ctx context.Context, toolName string, args json.RawMessage) (output string, files []FileRef, err error) {
	switch toolName {
	case "bash":
		output, err = t.handleBash(ctx, args)
	case "read_file":
		output, err = t.handleReadFile(ctx, args)
	case "write_file":
		output, err = t.handleWriteFile(ctx, args)
	case "ls":
		output, err = t.handleListDir(ctx, args)
	case "present_files":
		return t.handlePresentFiles(ctx, args)
	default:
		return "", nil, ErrUnknownTool
	}
	return output, nil, err
}

// IsWorkspaceTool returns true if the tool name is handled by this toolkit.
func (t *WorkspaceToolkit) IsWorkspaceTool(toolName string) bool {
	return IsWorkspaceTool(toolName)
}

// IsWorkspaceTool is a package-level convenience function.
func IsWorkspaceTool(name string) bool {
	switch name {
	case "bash", "read_file", "write_file", "ls", "present_files":
		return true
	}
	return false
}

// --------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------

func (t *WorkspaceToolkit) handleBash(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "invalid arguments: " + err.Error(), nil
	}
	if strings.TrimSpace(a.Command) == "" {
		return "command is required", nil
	}

	resp, err := t.client.SandboxExecute(ctx, t.sessionID, SandboxExecRequest{
		Command:   a.Command,
		TimeoutMs: int(t.bashTimeout.Milliseconds()),
	})
	if err != nil {
		return fmt.Sprintf("bash execution failed: %s", err), nil
	}

	var out strings.Builder
	if resp.Stdout != "" {
		out.WriteString(resp.Stdout)
	}
	if resp.Stderr != "" {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString("stderr: ")
		out.WriteString(resp.Stderr)
	}
	if resp.ExitCode != 0 {
		fmt.Fprintf(&out, "\n[exit code: %d]", resp.ExitCode)
	}
	if out.Len() == 0 {
		out.WriteString("(no output)")
	}
	return out.String(), nil
}

func (t *WorkspaceToolkit) handleReadFile(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path      string `json:"path"`
		StartLine *int   `json:"start_line,omitempty"`
		EndLine   *int   `json:"end_line,omitempty"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "invalid arguments: " + err.Error(), nil
	}
	if err := validateSandboxPath(a.Path); err != nil {
		return err.Error(), nil
	}

	content, err := t.client.SandboxReadFile(ctx, t.sessionID, a.Path)
	if err != nil {
		return fmt.Sprintf("failed to read file: %s", err), nil
	}

	if a.StartLine != nil || a.EndLine != nil {
		lines := strings.Split(content, "\n")
		start, end := 0, len(lines)
		if a.StartLine != nil && *a.StartLine > 0 {
			start = *a.StartLine - 1
			if start > len(lines) {
				start = len(lines)
			}
		}
		if a.EndLine != nil && *a.EndLine > 0 {
			end = *a.EndLine
			if end > len(lines) {
				end = len(lines)
			}
		}
		if start > end {
			start = end
		}
		content = strings.Join(lines[start:end], "\n")
	}

	return content, nil
}

func (t *WorkspaceToolkit) handleWriteFile(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Append  bool   `json:"append"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "invalid arguments: " + err.Error(), nil
	}
	if err := validateSandboxPath(a.Path); err != nil {
		return err.Error(), nil
	}

	if err := t.client.SandboxWriteFile(ctx, t.sessionID, a.Path, a.Content, a.Append); err != nil {
		return fmt.Sprintf("failed to write file: %s", err), nil
	}
	return fmt.Sprintf("File written to %s", a.Path), nil
}

func (t *WorkspaceToolkit) handleListDir(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "invalid arguments: " + err.Error(), nil
	}
	if err := validateSandboxPath(a.Path); err != nil {
		return err.Error(), nil
	}

	entries, err := t.client.SandboxListDir(ctx, t.sessionID, a.Path, 2)
	if err != nil {
		return fmt.Sprintf("failed to list directory: %s", err), nil
	}

	var out strings.Builder
	out.WriteString(a.Path)
	out.WriteString("/\n")
	for i, entry := range entries {
		name := filepath.Base(entry.Path)
		prefix := "├── "
		if i == len(entries)-1 {
			prefix = "└── "
		}
		out.WriteString(prefix)
		if entry.IsDir {
			out.WriteString(name + "/\n")
		} else {
			fmt.Fprintf(&out, "%s (%d bytes)\n", name, entry.Size)
		}
	}
	if len(entries) == 0 {
		out.WriteString("  (empty directory)\n")
	}
	return out.String(), nil
}

// presentableSources maps the `source` enum exposed to the LLM to the absolute sandbox dir it resolves to. Adding a new presentable surface (drive/, scratch/, ...) is a single map entry — no new tool, no schema migration, no allowlist string-prefix gotchas.
var presentableSources = map[string]string{
	"outputs": "/sandbox/session/outputs/",
	"uploads": "/sandbox/session/uploads/",
}

func presentableSourceKeys() []string {
	keys := make([]string, 0, len(presentableSources))
	for k := range presentableSources {
		keys = append(keys, k)
	}
	return keys
}

// IsValidPresentSource reports whether `source` is an accepted enum value. Exported so callers (aigent's executor mirror, future custom dispatchers) can run the same validation without re-implementing the map.
func IsValidPresentSource(source string) bool {
	_, ok := presentableSources[source]
	return ok
}

func (t *WorkspaceToolkit) handlePresentFiles(ctx context.Context, args json.RawMessage) (string, []FileRef, error) {
	var a struct {
		Source    string   `json:"source"`
		Filenames []string `json:"filenames"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "invalid arguments: " + err.Error(), nil, nil
	}
	root, ok := presentableSources[a.Source]
	if !ok {
		return fmt.Sprintf("source %q is not allowed; expected one of %v", a.Source, presentableSourceKeys()), nil, nil
	}
	if len(a.Filenames) == 0 {
		return "filenames is required and must not be empty", nil, nil
	}
	if len(a.Filenames) > 20 {
		return fmt.Sprintf("too many files: %d (max 20)", len(a.Filenames)), nil, nil
	}

	// Filenames are basenames relative to the source dir. Reject anything with separators or `..` so traversal can't sneak in via the relative side either.
	for _, name := range a.Filenames {
		if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			return fmt.Sprintf("filename %q must be a basename (no slashes, no '..')", name), nil, nil
		}
	}

	var refs []FileRef
	var presented []string

	for _, name := range a.Filenames {
		fp := root + name
		data, err := t.client.SandboxDownloadFile(ctx, t.sessionID, fp)
		if err != nil {
			continue
		}

		mimeType := detectMimeType(name)

		fileInfo, err := t.client.UploadFileFromReader(ctx, name, bytes.NewReader(data))
		if err != nil {
			continue
		}

		refs = append(refs, FileRef{
			ID:       fileInfo.ID,
			Filename: name,
			MimeType: mimeType,
			Size:     int64(len(data)),
		})
		presented = append(presented, fmt.Sprintf("- %s (%s)", name, fileInfo.ID))
	}

	if len(refs) == 0 {
		return "no files could be presented", nil, nil
	}

	output := fmt.Sprintf("Presented %d file(s):\n%s", len(refs), strings.Join(presented, "\n"))
	return output, refs, nil
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

func validateSandboxPath(p string) error {
	if p == "" {
		return fmt.Errorf("invalid path: path is empty")
	}
	cleaned := filepath.Clean(p)
	if !strings.HasPrefix(cleaned, "/sandbox/session") || strings.Contains(cleaned, "..") {
		return fmt.Errorf("invalid path: must start with /sandbox/session and not contain '..': %s", p)
	}
	return nil
}

func detectMimeType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".txt", ".log":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// --------------------------------------------------------------------
// Tool definitions
// --------------------------------------------------------------------

var workspaceToolDefs = []ToolDefinition{
	{
		Name:        "bash",
		Description: "Execute a bash command in the persistent workspace sandbox. The sandbox retains state (files, environment) across calls within the same session. Use /sandbox/session/outputs/ for files to return to the user.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"description": map[string]any{
					"type":        "string",
					"description": "A brief description of what this command does and why.",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "The bash command to execute.",
				},
			},
			"required": []string{"description", "command"},
		},
	},
	{
		Name:        "read_file",
		Description: "Read the contents of a file from the workspace. Supports optional line range selection.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the file (e.g. /sandbox/session/data.csv).",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "First line to read (1-based). If omitted, reads from beginning.",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "Last line to read (1-based, inclusive). If omitted, reads to end.",
				},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "write_file",
		Description: "Write content to a file in the workspace. Creates parent directories as needed.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path for the file (e.g. /sandbox/session/output.txt).",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file.",
				},
				"append": map[string]any{
					"type":        "boolean",
					"description": "If true, append to the file instead of overwriting. Defaults to false.",
				},
			},
			"required": []string{"path", "content"},
		},
	},
	{
		Name:        "ls",
		Description: "List files and directories at the given workspace path.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to list (e.g. /sandbox/session/).",
				},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "present_files",
		Description: "Present files to the user as downloadable artifacts. Pick the source by intent: 'outputs' for files you generated this turn, 'uploads' to re-display a file the user uploaded earlier in this session.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{
					"type":        "string",
					"enum":        []string{"outputs", "uploads"},
					"description": "Where the files live. 'outputs' = agent-generated deliverables under /sandbox/session/outputs/. 'uploads' = files the user uploaded earlier under /sandbox/session/uploads/.",
				},
				"filenames": map[string]any{
					"type":        "array",
					"description": "Basenames (no slashes, no '..') of files to present. Each must already exist inside the chosen source directory.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required": []string{"source", "filenames"},
		},
	},
}
