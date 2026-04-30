package skillbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsWorkspaceTool(t *testing.T) {
	yes := []string{"bash", "read_file", "write_file", "ls", "present_files"}
	for _, name := range yes {
		if !IsWorkspaceTool(name) {
			t.Errorf("IsWorkspaceTool(%q) = false, want true", name)
		}
	}
	no := []string{"ask_clarification", "think", "web_search", ""}
	for _, name := range no {
		if IsWorkspaceTool(name) {
			t.Errorf("IsWorkspaceTool(%q) = true, want false", name)
		}
	}
}

func TestToolDefinitions(t *testing.T) {
	client := New("http://localhost", "sk-test")
	toolkit := NewWorkspaceToolkit(client, "sess-1")
	defs := toolkit.ToolDefinitions()

	if len(defs) != 5 {
		t.Fatalf("got %d tool definitions, want 5", len(defs))
	}

	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
		if d.Description == "" {
			t.Errorf("tool %q has empty description", d.Name)
		}
		if d.Parameters == nil {
			t.Errorf("tool %q has nil parameters", d.Name)
		}
	}

	expected := []string{"bash", "read_file", "write_file", "ls", "present_files"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool definition for %q", name)
		}
	}
}

func TestHandle_UnknownTool(t *testing.T) {
	client := New("http://localhost", "sk-test")
	toolkit := NewWorkspaceToolkit(client, "sess-1")

	_, _, err := toolkit.Handle(context.Background(), "unknown_tool", json.RawMessage(`{}`))
	if err != ErrUnknownTool {
		t.Errorf("got err %v, want ErrUnknownTool", err)
	}
}

func TestHandle_Bash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sandbox/execute" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Session-ID") != "sess-1" {
			t.Errorf("X-Session-ID = %q, want %q", r.Header.Get("X-Session-ID"), "sess-1")
		}
		_ = json.NewEncoder(w).Encode(SandboxExecResponse{
			Stdout:   "hello world",
			Stderr:   "",
			ExitCode: 0,
		})
	}))
	defer srv.Close() //nolint:errcheck

	client := New(srv.URL, "sk-test")
	toolkit := NewWorkspaceToolkit(client, "sess-1")

	output, files, err := toolkit.Handle(context.Background(), "bash",
		json.RawMessage(`{"command": "echo hello world", "description": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files != nil {
		t.Errorf("bash should not return files")
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("output = %q, want to contain 'hello world'", output)
	}
}

func TestHandle_Bash_WithStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(SandboxExecResponse{
			Stdout:   "partial output",
			Stderr:   "warning: something",
			ExitCode: 1,
		})
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")
	output, _, _ := toolkit.Handle(context.Background(), "bash",
		json.RawMessage(`{"command": "failing cmd", "description": "test"}`))

	if !strings.Contains(output, "partial output") {
		t.Errorf("output missing stdout")
	}
	if !strings.Contains(output, "stderr: warning") {
		t.Errorf("output missing stderr")
	}
	if !strings.Contains(output, "[exit code: 1]") {
		t.Errorf("output missing exit code")
	}
}

func TestHandle_Bash_EmptyCommand(t *testing.T) {
	toolkit := NewWorkspaceToolkit(New("http://localhost", "sk-test"), "sess-1")
	output, _, _ := toolkit.Handle(context.Background(), "bash",
		json.RawMessage(`{"command": "", "description": "test"}`))
	if !strings.Contains(output, "command is required") {
		t.Errorf("output = %q, want 'command is required'", output)
	}
}

func TestHandle_ReadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Content string `json:"content"`
		}{Content: "line1\nline2\nline3\nline4\nline5"})
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")

	t.Run("full_file", func(t *testing.T) {
		output, _, _ := toolkit.Handle(context.Background(), "read_file",
			json.RawMessage(`{"path": "/sandbox/session/data.txt"}`))
		if !strings.Contains(output, "line1") || !strings.Contains(output, "line5") {
			t.Errorf("expected full file content, got: %q", output)
		}
	})

	t.Run("line_range", func(t *testing.T) {
		output, _, _ := toolkit.Handle(context.Background(), "read_file",
			json.RawMessage(`{"path": "/sandbox/session/data.txt", "start_line": 2, "end_line": 3}`))
		if output != "line2\nline3" {
			t.Errorf("output = %q, want %q", output, "line2\nline3")
		}
	})
}

func TestHandle_ReadFile_BadPath(t *testing.T) {
	toolkit := NewWorkspaceToolkit(New("http://localhost", "sk-test"), "sess-1")

	tests := []struct {
		name string
		path string
	}{
		{"path traversal", `/sandbox/session/../../etc/passwd`},
		{"outside sandbox", `/tmp/evil.txt`},
		{"empty", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"path": tt.path})
			output, _, _ := toolkit.Handle(context.Background(), "read_file", args)
			if !strings.Contains(output, "invalid path") {
				t.Errorf("output = %q, want 'invalid path'", output)
			}
		})
	}
}

func TestHandle_WriteFile(t *testing.T) {
	var gotPath, gotContent string
	var gotAppend bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotPath, _ = body["path"].(string)
		gotContent, _ = body["content"].(string)
		gotAppend, _ = body["append"].(bool)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")
	output, _, _ := toolkit.Handle(context.Background(), "write_file",
		json.RawMessage(`{"path": "/sandbox/session/out.txt", "content": "hello", "append": true}`))

	if gotPath != "/sandbox/session/out.txt" {
		t.Errorf("path = %q", gotPath)
	}
	if gotContent != "hello" {
		t.Errorf("content = %q", gotContent)
	}
	if !gotAppend {
		t.Error("append should be true")
	}
	if !strings.Contains(output, "File written") {
		t.Errorf("output = %q", output)
	}
}

func TestHandle_ListDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Entries []SandboxDirEntry `json:"entries"`
		}{
			Entries: []SandboxDirEntry{
				{Path: "/sandbox/session/scripts", IsDir: true, Size: 0},
				{Path: "/sandbox/session/data.csv", IsDir: false, Size: 1024},
			},
		})
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")
	output, _, _ := toolkit.Handle(context.Background(), "ls",
		json.RawMessage(`{"path": "/sandbox/session"}`))

	if !strings.Contains(output, "scripts/") {
		t.Errorf("output missing directory entry")
	}
	if !strings.Contains(output, "data.csv (1024 bytes)") {
		t.Errorf("output missing file entry with size")
	}
	if !strings.Contains(output, "├── ") || !strings.Contains(output, "└── ") {
		t.Errorf("output missing tree formatting")
	}
}

func TestHandle_PresentFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sandbox/read-file":
			_ = json.NewEncoder(w).Encode(struct {
				Content string `json:"content"`
			}{Content: "file content here"})
		case "/v1/files":
			_ = json.NewEncoder(w).Encode(FileInfo{
				ID:   "file-abc-123",
				Name: "report.pdf",
			})
		}
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")
	output, files, err := toolkit.Handle(context.Background(), "present_files",
		json.RawMessage(`{"source":"outputs","filenames":["report.pdf"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if files[0].ID != "file-abc-123" {
		t.Errorf("file ID = %q, want %q", files[0].ID, "file-abc-123")
	}
	if files[0].Filename != "report.pdf" {
		t.Errorf("filename = %q, want %q", files[0].Filename, "report.pdf")
	}
	if files[0].MimeType != "application/pdf" {
		t.Errorf("mime_type = %q, want %q", files[0].MimeType, "application/pdf")
	}
	if !strings.Contains(output, "Presented 1 file") {
		t.Errorf("output = %q", output)
	}
}

// Unknown source enum values must reject with a list of accepted sources so the LLM can self-correct on the next turn.
func TestHandle_PresentFiles_UnknownSourceRejected(t *testing.T) {
	toolkit := NewWorkspaceToolkit(New("http://localhost", "sk-test"), "sess-1")
	output, _, _ := toolkit.Handle(context.Background(), "present_files",
		json.RawMessage(`{"source":"scratch","filenames":["x.txt"]}`))
	if !strings.Contains(output, "is not allowed") {
		t.Errorf("rejection should explain the constraint; output = %q", output)
	}
	if !strings.Contains(output, "outputs") || !strings.Contains(output, "uploads") {
		t.Errorf("rejection should enumerate accepted sources; output = %q", output)
	}
}

// User-uploaded files mirrored to /sandbox/session/uploads/ must be presentable in a single tool call.
func TestHandle_PresentFiles_UploadsSourceAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sandbox/read-file":
			_ = json.NewEncoder(w).Encode(struct {
				Content string `json:"content"`
			}{Content: "user uploaded image bytes"})
		case "/v1/files":
			_ = json.NewEncoder(w).Encode(FileInfo{
				ID:   "file-upload-1",
				Name: "librarian.png",
			})
		}
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1")
	output, files, err := toolkit.Handle(context.Background(), "present_files",
		json.RawMessage(`{"source":"uploads","filenames":["librarian.png"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1; output=%q", len(files), output)
	}
	if files[0].Filename != "librarian.png" {
		t.Errorf("filename = %q, want %q", files[0].Filename, "librarian.png")
	}
	if !strings.Contains(output, "Presented 1 file") {
		t.Errorf("output = %q", output)
	}
}

// Filenames must be basenames; slashes and traversal segments are rejected at the parameter level so traversal can't sneak in via the relative side.
func TestHandle_PresentFiles_RejectsBadFilenames(t *testing.T) {
	toolkit := NewWorkspaceToolkit(New("http://localhost", "sk-test"), "sess-1")
	cases := []string{
		`{"source":"uploads","filenames":["../etc/passwd"]}`,
		`{"source":"uploads","filenames":["nested/file.png"]}`,
		`{"source":"uploads","filenames":[".."]}`,
		`{"source":"uploads","filenames":[""]}`,
	}
	for _, args := range cases {
		output, _, _ := toolkit.Handle(context.Background(), "present_files", json.RawMessage(args))
		if strings.Contains(output, "Presented") {
			t.Errorf("bad filename payload %q must be rejected; output = %q", args, output)
		}
	}
}

func TestHandle_PresentFiles_TooMany(t *testing.T) {
	toolkit := NewWorkspaceToolkit(New("http://localhost", "sk-test"), "sess-1")
	names := make([]string, 21)
	for i := range names {
		names[i] = "file.txt"
	}
	args, _ := json.Marshal(map[string]any{"source": "outputs", "filenames": names})
	output, _, _ := toolkit.Handle(context.Background(), "present_files", args)
	if !strings.Contains(output, "too many files") {
		t.Errorf("output = %q", output)
	}
}

func TestWithBashTimeout(t *testing.T) {
	var gotTimeout int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["timeout_ms"].(float64); ok {
			gotTimeout = int(v)
		}
		_ = json.NewEncoder(w).Encode(SandboxExecResponse{Stdout: "ok"})
	}))
	defer srv.Close() //nolint:errcheck

	toolkit := NewWorkspaceToolkit(New(srv.URL, "sk-test"), "sess-1",
		WithBashTimeout(30*time.Second))
	_, _, _ = toolkit.Handle(context.Background(), "bash",
		json.RawMessage(`{"command": "echo hi", "description": "test"}`))

	if gotTimeout != 30000 {
		t.Errorf("timeout_ms = %d, want 30000", gotTimeout)
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"report.pdf", "application/pdf"},
		{"image.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"data.csv", "text/csv"},
		{"config.json", "application/json"},
		{"readme.md", "text/markdown"},
		{"unknown.xyz", "application/octet-stream"},
	}
	for _, tt := range tests {
		got := detectMimeType(tt.file)
		if got != tt.want {
			t.Errorf("detectMimeType(%q) = %q, want %q", tt.file, got, tt.want)
		}
	}
}

func TestValidateSandboxPath(t *testing.T) {
	valid := []string{
		"/sandbox/session/data.txt",
		"/sandbox/session/outputs/report.pdf",
		"/sandbox/session/scripts/main.py",
	}
	for _, p := range valid {
		if err := validateSandboxPath(p); err != nil {
			t.Errorf("validateSandboxPath(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"",
		"/tmp/file.txt",
		"/sandbox/session/../../etc/passwd",
		"relative/path",
	}
	for _, p := range invalid {
		if err := validateSandboxPath(p); err == nil {
			t.Errorf("validateSandboxPath(%q) = nil, want error", p)
		}
	}
}
