package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/sandbox"
	"github.com/devs-group/skillbox/internal/skill"
)

// ---------------------------------------------------------------------------
// buildShellCommand
// ---------------------------------------------------------------------------

func TestBuildShellCommand_Python(t *testing.T) {
	loaded := &registry.LoadedSkill{
		Skill:           &skill.Skill{Lang: "python"},
		Entrypoint:      "main.py",
		HasRequirements: false,
	}
	got := buildShellCommand(loaded)
	want := "python /sandbox/scripts/main.py"
	if got != want {
		t.Errorf("buildShellCommand(python) = %q, want %q", got, want)
	}
}

func TestBuildShellCommand_PythonWithRequirements(t *testing.T) {
	loaded := &registry.LoadedSkill{
		Skill:           &skill.Skill{Lang: "python"},
		Entrypoint:      "main.py",
		HasRequirements: true,
	}
	got := buildShellCommand(loaded)
	if !strings.HasPrefix(got, "pip install") {
		t.Errorf("expected pip install prefix, got %q", got)
	}
	if !strings.Contains(got, "requirements.txt") {
		t.Errorf("expected requirements.txt in command, got %q", got)
	}
	if !strings.Contains(got, "PYTHONPATH=/tmp/deps") {
		t.Errorf("expected PYTHONPATH=/tmp/deps in command, got %q", got)
	}
	if !strings.HasSuffix(got, "python /sandbox/scripts/main.py") {
		t.Errorf("expected command to end with 'python /sandbox/scripts/main.py', got %q", got)
	}
}

func TestBuildShellCommand_Node(t *testing.T) {
	for _, lang := range []string{"node", "nodejs", "javascript"} {
		t.Run(lang, func(t *testing.T) {
			loaded := &registry.LoadedSkill{
				Skill:      &skill.Skill{Lang: lang},
				Entrypoint: "index.js",
			}
			got := buildShellCommand(loaded)
			want := "node /sandbox/scripts/index.js"
			if got != want {
				t.Errorf("buildShellCommand(%s) = %q, want %q", lang, got, want)
			}
		})
	}
}

func TestBuildShellCommand_Bash(t *testing.T) {
	loaded := &registry.LoadedSkill{
		Skill:      &skill.Skill{Lang: "bash"},
		Entrypoint: "run.sh",
	}
	got := buildShellCommand(loaded)
	want := "bash /sandbox/scripts/run.sh"
	if got != want {
		t.Errorf("buildShellCommand(bash) = %q, want %q", got, want)
	}
}

func TestBuildShellCommand_Shell(t *testing.T) {
	for _, lang := range []string{"shell", "sh"} {
		t.Run(lang, func(t *testing.T) {
			loaded := &registry.LoadedSkill{
				Skill:      &skill.Skill{Lang: lang},
				Entrypoint: "run.sh",
			}
			got := buildShellCommand(loaded)
			want := "sh /sandbox/scripts/run.sh"
			if got != want {
				t.Errorf("buildShellCommand(%s) = %q, want %q", lang, got, want)
			}
		})
	}
}

func TestBuildShellCommand_Default(t *testing.T) {
	loaded := &registry.LoadedSkill{
		Skill:      &skill.Skill{Lang: "ruby"},
		Entrypoint: "app.rb",
	}
	got := buildShellCommand(loaded)
	want := "/sandbox/scripts/app.rb"
	if got != want {
		t.Errorf("buildShellCommand(default) = %q, want %q", got, want)
	}
}

func TestBuildShellCommand_EmptyLang(t *testing.T) {
	loaded := &registry.LoadedSkill{
		Skill:      &skill.Skill{Lang: ""},
		Entrypoint: "run.bin",
	}
	got := buildShellCommand(loaded)
	want := "/sandbox/scripts/run.bin"
	if got != want {
		t.Errorf("buildShellCommand(empty lang) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// buildUploadFiles
// ---------------------------------------------------------------------------

func TestBuildUploadFiles_BasicSkillDir(t *testing.T) {
	// Create a temp directory with a few test files.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "helper.py"), []byte("# helper"), 0o644); err != nil {
		t.Fatal(err)
	}

	inputJSON := json.RawMessage(`{"key":"value"}`)

	files, err := buildUploadFiles(dir, inputJSON)
	if err != nil {
		t.Fatalf("buildUploadFiles: %v", err)
	}

	// Build a map of remote path -> FileUpload for easy lookup.
	byPath := make(map[string]sandbox.FileUpload)
	for _, f := range files {
		byPath[f.Path] = f
	}

	// Verify the skill files were mapped under /sandbox/scripts/.
	mainFile, ok := byPath["/sandbox/scripts/main.py"]
	if !ok {
		t.Fatal("missing /sandbox/scripts/main.py in upload list")
	}
	if string(mainFile.Content) != "print('hi')" {
		t.Errorf("main.py content = %q, want %q", mainFile.Content, "print('hi')")
	}
	// On most systems the permission will be 0o755. Verify mode is non-zero.
	if mainFile.Mode == 0 {
		t.Error("main.py mode should not be 0")
	}

	helperFile, ok := byPath["/sandbox/scripts/lib/helper.py"]
	if !ok {
		t.Fatal("missing /sandbox/scripts/lib/helper.py in upload list")
	}
	if string(helperFile.Content) != "# helper" {
		t.Errorf("helper.py content = %q, want %q", helperFile.Content, "# helper")
	}

	// Verify input.json is present.
	inputFile, ok := byPath["/sandbox/input.json"]
	if !ok {
		t.Fatal("missing /sandbox/input.json in upload list")
	}
	if string(inputFile.Content) != `{"key":"value"}` {
		t.Errorf("input.json content = %q, want %q", inputFile.Content, `{"key":"value"}`)
	}
	if inputFile.Mode != 0o644 {
		t.Errorf("input.json mode = %#o, want %#o", inputFile.Mode, 0o644)
	}

	// Verify .keep placeholder files.
	keepOut, ok := byPath["/sandbox/out/.keep"]
	if !ok {
		t.Fatal("missing /sandbox/out/.keep in upload list")
	}
	if len(keepOut.Content) != 0 {
		t.Errorf("out/.keep should be empty, got %d bytes", len(keepOut.Content))
	}

	keepFiles, ok := byPath["/sandbox/out/files/.keep"]
	if !ok {
		t.Fatal("missing /sandbox/out/files/.keep in upload list")
	}
	if len(keepFiles.Content) != 0 {
		t.Errorf("out/files/.keep should be empty, got %d bytes", len(keepFiles.Content))
	}
}

func TestBuildUploadFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	inputJSON := json.RawMessage(`{}`)

	files, err := buildUploadFiles(dir, inputJSON)
	if err != nil {
		t.Fatalf("buildUploadFiles: %v", err)
	}

	// Should contain exactly: input.json + 2 .keep placeholders = 3 files.
	if len(files) != 3 {
		t.Errorf("expected 3 files for empty skill dir, got %d", len(files))
	}
}

func TestBuildUploadFiles_NonexistentDir(t *testing.T) {
	_, err := buildUploadFiles("/nonexistent/dir/"+t.Name(), nil)
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// isBlockedEnvVar
// ---------------------------------------------------------------------------

func TestIsBlockedEnvVar_ExactMatches(t *testing.T) {
	blocked := []string{
		"PATH", "HOME", "LD_PRELOAD", "LD_LIBRARY_PATH",
		"PYTHONPATH", "NODE_PATH", "NODE_OPTIONS",
	}
	for _, key := range blocked {
		t.Run(key, func(t *testing.T) {
			if !isBlockedEnvVar(key) {
				t.Errorf("isBlockedEnvVar(%q) = false, want true", key)
			}
		})
	}
}

func TestIsBlockedEnvVar_Prefixes(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"SANDBOX_INPUT", true},
		{"SANDBOX_OUTPUT", true},
		{"sandbox_lower", true},  // case-insensitive prefix check
		{"Sandbox_Mixed", true},  // case-insensitive prefix check
		{"SKILL_INSTRUCTIONS", true},
		{"SKILL_CUSTOM", true},
		{"skill_lower", true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isBlockedEnvVar(tt.key)
			if got != tt.want {
				t.Errorf("isBlockedEnvVar(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsBlockedEnvVar_AllowedVars(t *testing.T) {
	allowed := []string{
		"MY_API_KEY",
		"DATABASE_URL",
		"CUSTOM_VAR",
		"FOO",
		"BAR_BAZ",
	}
	for _, key := range allowed {
		t.Run(key, func(t *testing.T) {
			if isBlockedEnvVar(key) {
				t.Errorf("isBlockedEnvVar(%q) = true, want false", key)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// shortID
// ---------------------------------------------------------------------------

func TestShortID_Truncation(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"long ID", "abcdefghijklmnop", "abcdefghijkl"},
		{"exactly 12", "abcdefghijkl", "abcdefghijkl"},
		{"short ID", "abc", "abc"},
		{"empty", "", ""},
		{"13 chars", "abcdefghijklm", "abcdefghijkl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortID(tt.id)
			if got != tt.want {
				t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateString
// ---------------------------------------------------------------------------

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxBytes int64
		want     string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello"},
		{"zero max", "hello", 0, ""},
		{"empty string", "", 10, ""},
		{"one byte max", "hello", 1, "h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxBytes)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxBytes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RunResult.setError
// ---------------------------------------------------------------------------

func TestRunResult_SetError(t *testing.T) {
	r := &RunResult{}

	// Setting a non-empty message should populate Error.
	r.setError("something went wrong")
	if r.Error == nil {
		t.Fatal("expected Error to be non-nil after setError")
	}
	if *r.Error != "something went wrong" {
		t.Errorf("Error = %q, want %q", *r.Error, "something went wrong")
	}

	// Setting an empty message should clear Error.
	r.setError("")
	if r.Error != nil {
		t.Errorf("expected Error to be nil after setError(\"\"), got %q", *r.Error)
	}
}

// ---------------------------------------------------------------------------
// pollExecD — uses a real HTTP test server to simulate Ping behavior
// ---------------------------------------------------------------------------

func TestPollExecD_ImmediateSuccess(t *testing.T) {
	// Server that always returns 200 on /ping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := sandbox.New("http://unused", "", srv.Client())

	ctx := context.Background()
	err := pollExecD(ctx, client, srv.URL, 50*time.Millisecond, 2*time.Second)
	if err != nil {
		t.Fatalf("pollExecD: unexpected error: %v", err)
	}
}

func TestPollExecD_SucceedsAfterRetries(t *testing.T) {
	// Server that fails the first 3 pings, then succeeds.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := sandbox.New("http://unused", "", srv.Client())

	ctx := context.Background()
	err := pollExecD(ctx, client, srv.URL, 50*time.Millisecond, 5*time.Second)
	if err != nil {
		t.Fatalf("pollExecD: unexpected error after retries: %v", err)
	}
	if callCount < 4 {
		t.Errorf("expected at least 4 calls, got %d", callCount)
	}
}

func TestPollExecD_Timeout(t *testing.T) {
	// Server that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := sandbox.New("http://unused", "", srv.Client())

	ctx := context.Background()
	err := pollExecD(ctx, client, srv.URL, 50*time.Millisecond, 200*time.Millisecond)
	if err == nil {
		t.Fatal("pollExecD: expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "did not become ready") {
		t.Errorf("error = %q, want it to mention 'did not become ready'", err.Error())
	}
}

func TestPollExecD_ContextCancelled(t *testing.T) {
	// Server that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := sandbox.New("http://unused", "", srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := pollExecD(ctx, client, srv.URL, 50*time.Millisecond, 10*time.Second)
	if err == nil {
		t.Fatal("pollExecD: expected context cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("error = %q, want it to mention 'context cancelled'", err.Error())
	}
}
