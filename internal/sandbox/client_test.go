package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer creates an httptest.Server that dispatches to handler.
// It returns the server and a Client whose lifecycleURL points to it.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cl := New(srv.URL, "test-api-key", srv.Client())
	return srv, cl
}

// sandboxJSON builds a minimal valid sandbox JSON blob.
func sandboxJSON(id, state string) string {
	return fmt.Sprintf(`{
		"id": %q,
		"status": {"state": %q},
		"expires_at": "2026-03-04T00:00:00Z",
		"created_at": "2026-03-03T00:00:00Z",
		"metadata": {"env": "test"}
	}`, id, state)
}

// ---------------------------------------------------------------------------
// 1. CreateSandbox
// ---------------------------------------------------------------------------

func TestCreateSandbox_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("OPEN-SANDBOX-API-KEY") != "test-api-key" {
			t.Errorf("api key header = %q, want %q", r.Header.Get("OPEN-SANDBOX-API-KEY"), "test-api-key")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		var body sandboxBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body.Image.URI != "python:3.12-slim" {
			t.Errorf("image.uri = %q, want %q", body.Image.URI, "python:3.12-slim")
		}
		if body.Timeout != 300 {
			t.Errorf("timeout = %d, want 300", body.Timeout)
		}
		if body.Env["FOO"] != "bar" {
			t.Errorf("env[FOO] = %q, want %q", body.Env["FOO"], "bar")
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, sandboxJSON("sb-123", "Pending")) //nolint:errcheck
	})

	_, cl := newTestServer(t, mux)

	resp, err := cl.CreateSandbox(context.Background(), SandboxOpts{
		Image:   "python:3.12-slim",
		Timeout: 300,
		Env:     map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "sb-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "sb-123")
	}
	if resp.State != "Pending" {
		t.Errorf("State = %q, want %q", resp.State, "Pending")
	}
	if resp.Metadata["env"] != "test" {
		t.Errorf("Metadata[env] = %q, want %q", resp.Metadata["env"], "test")
	}
}

func TestCreateSandbox_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal failure") //nolint:errcheck
	})

	_, cl := newTestServer(t, mux)

	_, err := cl.CreateSandbox(context.Background(), SandboxOpts{Image: "python:3.12-slim"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to contain status code 500", err.Error())
	}
	if !strings.Contains(err.Error(), "internal failure") {
		t.Errorf("error = %q, want it to contain response body", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 2. GetSandbox
// ---------------------------------------------------------------------------

func TestGetSandbox_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-456", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-456", "Running")) //nolint:errcheck
	})

	_, cl := newTestServer(t, mux)

	resp, err := cl.GetSandbox(context.Background(), "sb-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "sb-456" {
		t.Errorf("ID = %q, want %q", resp.ID, "sb-456")
	}
	if resp.State != "Running" {
		t.Errorf("State = %q, want %q", resp.State, "Running")
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
	if resp.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestGetSandbox_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "sandbox not found")
	})

	_, cl := newTestServer(t, mux)

	_, err := cl.GetSandbox(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to contain 404", err.Error())
	}
}

func TestGetSandbox_EscapesID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/id%2Fwith%2Fslash", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("id/with/slash", "Running"))
	})

	_, cl := newTestServer(t, mux)

	resp, err := cl.GetSandbox(context.Background(), "id/with/slash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "id/with/slash" {
		t.Errorf("ID = %q, want %q", resp.ID, "id/with/slash")
	}
}

// ---------------------------------------------------------------------------
// 3. ListSandboxes
// ---------------------------------------------------------------------------

func TestListSandboxes_MetadataParams(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		values := r.URL.Query()["metadata"]
		if len(values) == 0 {
			t.Error("expected metadata query params, got none")
		}
		found := map[string]bool{}
		for _, v := range values {
			found[v] = true
		}
		if !found["env=prod"] {
			t.Error("missing metadata param env=prod")
		}
		if !found["team=backend"] {
			t.Error("missing metadata param team=backend")
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "[%s,%s]", sandboxJSON("sb-1", "Running"), sandboxJSON("sb-2", "Pending")) //nolint:errcheck
	})

	_, cl := newTestServer(t, mux)

	list, err := cl.ListSandboxes(context.Background(), map[string]string{
		"env":  "prod",
		"team": "backend",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].ID != "sb-1" {
		t.Errorf("list[0].ID = %q, want %q", list[0].ID, "sb-1")
	}
	if list[1].ID != "sb-2" {
		t.Errorf("list[1].ID = %q, want %q", list[1].ID, "sb-2")
	}
}

func TestListSandboxes_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "[]")
	})

	_, cl := newTestServer(t, mux)

	list, err := cl.ListSandboxes(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(list) = %d, want 0", len(list))
	}
}

// ---------------------------------------------------------------------------
// 4. DeleteSandbox
// ---------------------------------------------------------------------------

func TestDeleteSandbox_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, cl := newTestServer(t, mux)

	err := cl.DeleteSandbox(context.Background(), "sb-del")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteSandbox_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-gone", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	})

	_, cl := newTestServer(t, mux)

	err := cl.DeleteSandbox(context.Background(), "sb-gone")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to contain 404", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 5. GetEndpoint
// ---------------------------------------------------------------------------

func TestGetEndpoint_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-ep/endpoints/8080", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(endpointWire{ //nolint:errcheck
			Host:    "sandbox-host.example.com",
			Port:    8080,
			URL:     "http://sandbox-host.example.com:8080",
			Headers: map[string]string{"X-Token": "abc123"},
		})
	})

	_, cl := newTestServer(t, mux)

	ep, err := cl.GetEndpoint(context.Background(), "sb-ep", 8080)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Host != "sandbox-host.example.com" {
		t.Errorf("Host = %q, want %q", ep.Host, "sandbox-host.example.com")
	}
	if ep.Port != 8080 {
		t.Errorf("Port = %d, want 8080", ep.Port)
	}
	if ep.URL != "http://sandbox-host.example.com:8080" {
		t.Errorf("URL = %q, want %q", ep.URL, "http://sandbox-host.example.com:8080")
	}
	if ep.Headers["X-Token"] != "abc123" {
		t.Errorf("Headers[X-Token] = %q, want %q", ep.Headers["X-Token"], "abc123")
	}
}

// ---------------------------------------------------------------------------
// 6. DiscoverExecD
// ---------------------------------------------------------------------------

func TestDiscoverExecD_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-exec/endpoints/44772", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(endpointWire{ //nolint:errcheck
			Host:    "exec-host",
			Port:    ExecDPort,
			URL:     "http://exec-host:44772",
			Headers: map[string]string{"Authorization": "Bearer tok"},
		})
	})

	_, cl := newTestServer(t, mux)

	u, headers, err := cl.DiscoverExecD(context.Background(), "sb-exec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != "http://exec-host:44772" {
		t.Errorf("url = %q, want %q", u, "http://exec-host:44772")
	}
	if headers["Authorization"] != "Bearer tok" {
		t.Errorf("headers[Authorization] = %q, want %q", headers["Authorization"], "Bearer tok")
	}
}

func TestDiscoverExecD_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-exec/endpoints/44772", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "endpoint not found")
	})

	_, cl := newTestServer(t, mux)

	_, _, err := cl.DiscoverExecD(context.Background(), "sb-exec")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// 7. WaitReady
// ---------------------------------------------------------------------------

func TestWaitReady_AlreadyRunning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-run", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-run", "Running"))
	})

	_, cl := newTestServer(t, mux)

	resp, err := cl.WaitReady(context.Background(), "sb-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.State != "Running" {
		t.Errorf("State = %q, want %q", resp.State, "Running")
	}
}

func TestWaitReady_TransitionsToRunning(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-wait", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		state := "Pending"
		if callCount >= 3 {
			state = "Running"
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-wait", state))
	})

	_, cl := newTestServer(t, mux)

	resp, err := cl.WaitReady(context.Background(), "sb-wait")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.State != "Running" {
		t.Errorf("State = %q, want %q", resp.State, "Running")
	}
	if callCount < 3 {
		t.Errorf("callCount = %d, want >= 3", callCount)
	}
}

func TestWaitReady_FailedState(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-fail", "Failed"))
	})

	_, cl := newTestServer(t, mux)

	_, err := cl.WaitReady(context.Background(), "sb-fail")
	if err == nil {
		t.Fatal("expected error for Failed state, got nil")
	}
	if !strings.Contains(err.Error(), "terminal state") {
		t.Errorf("error = %q, want it to mention terminal state", err.Error())
	}
	if !strings.Contains(err.Error(), "Failed") {
		t.Errorf("error = %q, want it to mention Failed", err.Error())
	}
}

func TestWaitReady_TerminatedState(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-term", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-term", "Terminated"))
	})

	_, cl := newTestServer(t, mux)

	_, err := cl.WaitReady(context.Background(), "sb-term")
	if err == nil {
		t.Fatal("expected error for Terminated state, got nil")
	}
	if !strings.Contains(err.Error(), "Terminated") {
		t.Errorf("error = %q, want it to mention Terminated", err.Error())
	}
}

func TestWaitReady_ContextTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sb-slow", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sandboxJSON("sb-slow", "Pending"))
	})

	_, cl := newTestServer(t, mux)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	_, err := cl.WaitReady(ctx, "sb-slow")
	if err == nil {
		t.Fatal("expected error on context timeout, got nil")
	}
	if !strings.Contains(err.Error(), "waiting for sandbox") {
		t.Errorf("error = %q, want it to mention waiting for sandbox", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 8. Ping
// ---------------------------------------------------------------------------

func TestPing_Success(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("path = %q, want /ping", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "pong")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.Ping(context.Background(), execd.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPing_ErrorStatus(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "not ready")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.Ping(context.Background(), execd.URL)
	if err == nil {
		t.Fatal("expected error for 503 status, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to contain 503", err.Error())
	}
}

func TestPing_TrailingSlashTrimmed(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("path = %q, want /ping", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.Ping(context.Background(), execd.URL+"/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 9. UploadFiles
// ---------------------------------------------------------------------------

func TestUploadFiles_Success(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/upload" {
			t.Errorf("path = %q, want /files/upload", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parsing content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q, want multipart/form-data", mediaType)
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		var metadatas []string
		var files []string
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading part: %v", err)
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "metadata":
				metadatas = append(metadatas, string(data))
			case "file":
				files = append(files, string(data))
			default:
				t.Errorf("unexpected form field: %q", part.FormName())
			}
		}

		if len(metadatas) != 2 {
			t.Errorf("metadata parts = %d, want 2", len(metadatas))
		}
		if len(files) != 2 {
			t.Errorf("file parts = %d, want 2", len(files))
		}

		// Verify first metadata.
		var meta fileMetaWire
		if err := json.Unmarshal([]byte(metadatas[0]), &meta); err != nil {
			t.Fatalf("unmarshalling metadata: %v", err)
		}
		if meta.Path != "/app/main.py" {
			t.Errorf("meta.Path = %q, want /app/main.py", meta.Path)
		}
		if meta.Mode != 0o644 {
			t.Errorf("meta.Mode = %o, want 644", meta.Mode)
		}

		if files[0] != "print('hello')" {
			t.Errorf("file[0] = %q, want %q", files[0], "print('hello')")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.UploadFiles(context.Background(), execd.URL, []FileUpload{
		{Path: "/app/main.py", Content: []byte("print('hello')"), Mode: 0o644},
		{Path: "/app/util.py", Content: []byte("x = 1"), Mode: 0o644},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadFiles_AcceptsCreated(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.UploadFiles(context.Background(), execd.URL, []FileUpload{
		{Path: "/app/f.txt", Content: []byte("data"), Mode: 0o644},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadFiles_AcceptsNoContent(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.UploadFiles(context.Background(), execd.URL, []FileUpload{
		{Path: "/app/f.txt", Content: []byte("data"), Mode: 0o644},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadFiles_ServerError(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "disk full")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	err := cl.UploadFiles(context.Background(), execd.URL, []FileUpload{
		{Path: "/f", Content: []byte("x"), Mode: 0o644},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to contain 500", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 10. RunCommand — bare JSON and data:-prefixed SSE
// ---------------------------------------------------------------------------

func TestRunCommand_BareJSON(t *testing.T) {
	sseBody := strings.Join([]string{
		`{"type":"stdout","data":"hello world\n"}`,
		"",
		`{"type":"stderr","data":"warn: something\n"}`,
		"",
		`{"type":"execution_complete","exitCode":0,"durationMs":150}`,
		"",
	}, "\n")

	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			t.Errorf("path = %q, want /command", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		var cmd cmdReqWire
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if cmd.Command != "echo hello" {
			t.Errorf("command = %q, want %q", cmd.Command, "echo hello")
		}
		if cmd.Cwd != "/app" {
			t.Errorf("cwd = %q, want /app", cmd.Cwd)
		}
		if cmd.Timeout != 30 {
			t.Errorf("timeout = %d, want 30", cmd.Timeout)
		}
		if cmd.Background {
			t.Error("background should be false")
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	result, err := cl.RunCommand(context.Background(), execd.URL, "echo hello", "/app", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello world\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello world\n")
	}
	if result.Stderr != "warn: something\n" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "warn: something\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Duration != 150*time.Millisecond {
		t.Errorf("Duration = %v, want 150ms", result.Duration)
	}
}

func TestRunCommand_DataPrefixed(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"type":"stdout","data":"line1\n"}`,
		"",
		`data:{"type":"stdout","data":"line2\n"}`,
		"",
		`data: {"type":"error","data":"something broke"}`,
		"",
		`data: {"type":"execution_complete","exitCode":1,"durationMs":200}`,
		"",
	}, "\n")

	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	result, err := cl.RunCommand(context.Background(), execd.URL, "failing cmd", "/", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "line1\nline2\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "line1\nline2\n")
	}
	if result.Error != "something broke" {
		t.Errorf("Error = %q, want %q", result.Error, "something broke")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Duration != 200*time.Millisecond {
		t.Errorf("Duration = %v, want 200ms", result.Duration)
	}
}

func TestRunCommand_ServerError(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad command")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	_, err := cl.RunCommand(context.Background(), execd.URL, "bad", "/", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain 400", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 11. DownloadFile
// ---------------------------------------------------------------------------

func TestDownloadFile_Success(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/download" {
			t.Errorf("path = %q, want /files/download", r.URL.Path)
		}
		if r.URL.Query().Get("path") != "/app/output.txt" {
			t.Errorf("path param = %q, want /app/output.txt", r.URL.Query().Get("path"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "file content here")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	rc, err := cl.DownloadFile(context.Background(), execd.URL, "/app/output.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close() //nolint:errcheck

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(data) != "file content here" {
		t.Errorf("body = %q, want %q", string(data), "file content here")
	}
}

func TestDownloadFile_NotFound(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "file not found")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	_, err := cl.DownloadFile(context.Background(), execd.URL, "/missing.txt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to contain 404", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 12. SearchFiles
// ---------------------------------------------------------------------------

func TestSearchFiles_Success(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/search" {
			t.Errorf("path = %q, want /files/search", r.URL.Path)
		}
		if r.URL.Query().Get("path") != "/app" {
			t.Errorf("path param = %q, want /app", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("pattern") != "*.py" {
			t.Errorf("pattern param = %q, want *.py", r.URL.Query().Get("pattern"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]fileInfoWire{ //nolint:errcheck
			{Path: "/app/main.py", Size: 256, ModifiedAt: now},
			{Path: "/app/util.py", Size: 128, ModifiedAt: now},
		})
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	files, err := cl.SearchFiles(context.Background(), execd.URL, "/app", "*.py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].Path != "/app/main.py" {
		t.Errorf("files[0].Path = %q, want /app/main.py", files[0].Path)
	}
	if files[0].Size != 256 {
		t.Errorf("files[0].Size = %d, want 256", files[0].Size)
	}
	if files[1].Path != "/app/util.py" {
		t.Errorf("files[1].Path = %q, want /app/util.py", files[1].Path)
	}
	if files[0].ModifiedAt.IsZero() {
		t.Error("files[0].ModifiedAt should not be zero")
	}
}

func TestSearchFiles_Empty(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "[]")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	files, err := cl.SearchFiles(context.Background(), execd.URL, "/app", "*.rs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("len(files) = %d, want 0", len(files))
	}
}

func TestSearchFiles_ServerError(t *testing.T) {
	execd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "search failed")
	}))
	defer execd.Close()

	cl := New("http://unused", "key", execd.Client())

	_, err := cl.SearchFiles(context.Background(), execd.URL, "/app", "*")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to contain 500", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 13. Error handling
// ---------------------------------------------------------------------------

func TestErrorHandling_NonExpectedStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"bad request", http.StatusBadRequest, "invalid json"},
		{"unauthorized", http.StatusUnauthorized, "bad api key"},
		{"forbidden", http.StatusForbidden, "not allowed"},
		{"conflict", http.StatusConflict, "sandbox already exists"},
		{"too many requests", http.StatusTooManyRequests, "rate limited"},
		{"internal server error", http.StatusInternalServerError, "server crash"},
		{"bad gateway", http.StatusBadGateway, "upstream down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/sandboxes/sb-err", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.body)
			})

			_, cl := newTestServer(t, mux)

			_, err := cl.GetSandbox(context.Background(), "sb-err")
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tt.statusCode)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.statusCode)) {
				t.Errorf("error = %q, want it to contain status code %d", err.Error(), tt.statusCode)
			}
			if !strings.Contains(err.Error(), tt.body) {
				t.Errorf("error = %q, want it to contain response body %q", err.Error(), tt.body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// New client
// ---------------------------------------------------------------------------

func TestNew_TrimsTrailingSlash(t *testing.T) {
	cl := New("http://example.com/v1/", "key", nil)
	if cl.lifecycleURL != "http://example.com/v1" {
		t.Errorf("lifecycleURL = %q, want trailing slash trimmed", cl.lifecycleURL)
	}
}

func TestNew_DefaultHTTPClient(t *testing.T) {
	cl := New("http://example.com", "key", nil)
	if cl.httpClient == nil {
		t.Fatal("httpClient should not be nil when none provided")
	}
	if cl.httpClient.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", cl.httpClient.Timeout)
	}
}

func TestNew_CustomHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 60 * time.Second}
	cl := New("http://example.com", "key", custom)
	if cl.httpClient != custom {
		t.Error("httpClient should be the custom client provided")
	}
}

// ---------------------------------------------------------------------------
// parseSSEStream edge cases
// ---------------------------------------------------------------------------

func TestParseSSEStream_EmptyInput(t *testing.T) {
	result, err := parseSSEStream(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "" {
		t.Errorf("Stdout = %q, want empty", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestParseSSEStream_InvalidJSON(t *testing.T) {
	// Invalid JSON lines should be silently skipped.
	input := "not valid json\n\n" + `{"type":"stdout","data":"ok\n"}` + "\n\n"
	result, err := parseSSEStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "ok\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok\n")
	}
}

func TestParseSSEStream_NoTrailingNewlines(t *testing.T) {
	// Events without trailing "\n\n" should still be processed.
	input := `{"type":"stdout","data":"last line"}`
	result, err := parseSSEStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "last line" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "last line")
	}
}

func TestParseSSEStream_MixedPrefixed(t *testing.T) {
	// Mix of data:-prefixed and bare JSON.
	input := strings.Join([]string{
		`{"type":"stdout","data":"bare\n"}`,
		"",
		`data: {"type":"stdout","data":"prefixed\n"}`,
		"",
		`data:{"type":"stderr","data":"err\n"}`,
		"",
	}, "\n")
	result, err := parseSSEStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "bare\nprefixed\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "bare\nprefixed\n")
	}
	if result.Stderr != "err\n" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "err\n")
	}
}

// ---------------------------------------------------------------------------
// parseTime
// ---------------------------------------------------------------------------

func TestParseTime_Formats(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2026-03-03T12:00:00Z", true},
		{"2026-03-03T12:00:00.123456789Z", true},
		{"2026-03-03T12:00:00+00:00", true},
		{"", true}, // empty returns zero time
		{"not-a-time", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts, err := parseTime(tt.input)
			if tt.valid && err != nil {
				t.Errorf("parseTime(%q) unexpected error: %v", tt.input, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("parseTime(%q) expected error, got nil", tt.input)
			}
			if tt.input == "" && !ts.IsZero() {
				t.Errorf("parseTime(\"\") should return zero time, got %v", ts)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExecDPort constant
// ---------------------------------------------------------------------------

func TestExecDPort_Value(t *testing.T) {
	if ExecDPort != 44772 {
		t.Errorf("ExecDPort = %d, want 44772", ExecDPort)
	}
}
