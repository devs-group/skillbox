package skillbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------
// TestNew_WithEnvVar
// --------------------------------------------------------------------

func TestNew_WithEnvVar(t *testing.T) {
	const want = "sk-env-key-12345"
	t.Setenv("SKILLBOX_API_KEY", want)

	client := New("http://localhost:8080", "")
	if client.apiKey != want {
		t.Fatalf("expected apiKey %q from env, got %q", want, client.apiKey)
	}
	if client.httpClient != http.DefaultClient {
		t.Fatal("expected default http client")
	}
}

func TestNew_ExplicitKeyOverridesEnv(t *testing.T) {
	t.Setenv("SKILLBOX_API_KEY", "sk-from-env")
	const explicit = "sk-explicit"

	client := New("http://localhost:8080", explicit)
	if client.apiKey != explicit {
		t.Fatalf("expected apiKey %q, got %q", explicit, client.apiKey)
	}
}

func TestNew_WithOptions(t *testing.T) {
	hc := &http.Client{Timeout: 42 * time.Second}
	client := New("http://localhost:8080/", "sk-key",
		WithTenant("tenant-99"),
		WithHTTPClient(hc),
	)

	if client.tenantID != "tenant-99" {
		t.Fatalf("expected tenantID %q, got %q", "tenant-99", client.tenantID)
	}
	if client.httpClient != hc {
		t.Fatal("expected custom http client")
	}
	// Trailing slash should be trimmed.
	if client.baseURL != "http://localhost:8080" {
		t.Fatalf("expected trimmed baseURL, got %q", client.baseURL)
	}
}

// --------------------------------------------------------------------
// TestRun_Success
// --------------------------------------------------------------------

func TestRun_Success(t *testing.T) {
	want := RunResult{
		ExecutionID: "exec-abc-123",
		Status:      "completed",
		Output:      json.RawMessage(`{"mean": 2}`),
		FilesURL:    "http://example.com/files.tar.gz",
		FilesList:   []string{"result.csv"},
		Logs:        "processing...\ndone.",
		DurationMs:  1500,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/executions" {
			t.Errorf("expected path /v1/executions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("expected Bearer sk-test, got %q", got)
		}
		if got := r.Header.Get("X-Tenant-ID"); got != "tenant-1" {
			t.Errorf("expected X-Tenant-ID tenant-1, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", got)
		}

		// Validate body.
		var req RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Skill != "data-analysis" {
			t.Errorf("expected skill data-analysis, got %s", req.Skill)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test", WithTenant("tenant-1"))
	result, err := client.Run(context.Background(), RunRequest{
		Skill: "data-analysis",
		Input: json.RawMessage(`{"data": [1, 2, 3]}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExecutionID != want.ExecutionID {
		t.Errorf("ExecutionID: got %q, want %q", result.ExecutionID, want.ExecutionID)
	}
	if result.Status != want.Status {
		t.Errorf("Status: got %q, want %q", result.Status, want.Status)
	}
	// Compare JSON semantically — encoding may compact whitespace.
	var gotOutput, wantOutput interface{}
	_ = json.Unmarshal(result.Output, &gotOutput)
	_ = json.Unmarshal(want.Output, &wantOutput)
	gotBytes, _ := json.Marshal(gotOutput)
	wantBytes, _ := json.Marshal(wantOutput)
	if string(gotBytes) != string(wantBytes) {
		t.Errorf("Output: got %s, want %s", result.Output, want.Output)
	}
	if !result.HasFiles() {
		t.Error("expected HasFiles() to return true")
	}
	if result.DurationMs != want.DurationMs {
		t.Errorf("DurationMs: got %d, want %d", result.DurationMs, want.DurationMs)
	}
}

// --------------------------------------------------------------------
// TestRun_WithInputFiles — verifies input_files are serialized in the request
// --------------------------------------------------------------------

func TestRun_WithInputFiles(t *testing.T) {
	want := RunResult{
		ExecutionID: "exec-with-files",
		Status:      "completed",
		Output:      json.RawMessage(`{"processed": true}`),
		DurationMs:  800,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Skill != "xlsx" {
			t.Errorf("expected skill xlsx, got %s", req.Skill)
		}
		if len(req.InputFiles) != 2 {
			t.Fatalf("expected 2 input_files, got %d", len(req.InputFiles))
		}
		if req.InputFiles[0] != "file-id-aaa" {
			t.Errorf("InputFiles[0] = %q, want %q", req.InputFiles[0], "file-id-aaa")
		}
		if req.InputFiles[1] != "file-id-bbb" {
			t.Errorf("InputFiles[1] = %q, want %q", req.InputFiles[1], "file-id-bbb")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test", WithTenant("tenant-1"))
	result, err := client.Run(context.Background(), RunRequest{
		Skill:      "xlsx",
		Input:      json.RawMessage(`{"action": "update"}`),
		InputFiles: []string{"file-id-aaa", "file-id-bbb"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionID != want.ExecutionID {
		t.Errorf("ExecutionID: got %q, want %q", result.ExecutionID, want.ExecutionID)
	}
	if result.Status != want.Status {
		t.Errorf("Status: got %q, want %q", result.Status, want.Status)
	}
}

// --------------------------------------------------------------------
// TestRun_FailedExecution
// --------------------------------------------------------------------

func TestRun_FailedExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RunResult{
			ExecutionID: "exec-fail-456",
			Status:      "failed",
			Error:       "skill exited with code 1",
			DurationMs:  200,
		})
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	result, err := client.Run(context.Background(), RunRequest{Skill: "broken-skill"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("expected status failed, got %q", result.Status)
	}
	if result.Error != "skill exited with code 1" {
		t.Errorf("unexpected error message: %q", result.Error)
	}
	if result.HasFiles() {
		t.Error("expected HasFiles() to return false for failed execution")
	}
}

// --------------------------------------------------------------------
// TestRun_Timeout
// --------------------------------------------------------------------

func TestRun_Timeout(t *testing.T) {
	// Block the handler until the test is done so the context cancels the request.
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-gate:
		}
	}))
	defer func() {
		close(gate) // unblock any lingering handler goroutines
		srv.Close()
	}()

	client := New(srv.URL, "sk-test")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Run(ctx, RunRequest{Skill: "slow-skill"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline exceeded, got: %v", err)
	}
}

// --------------------------------------------------------------------
// TestListSkills
// --------------------------------------------------------------------

func TestListSkills(t *testing.T) {
	want := []Skill{
		{Name: "data-analysis", Version: "1.0.0", Description: "Analyze datasets"},
		{Name: "web-scraper", Version: "2.1.0", Description: "Scrape web pages"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/skills" {
			t.Errorf("expected path /v1/skills, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	skills, err := client.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != len(want) {
		t.Fatalf("expected %d skills, got %d", len(want), len(skills))
	}
	for i, s := range skills {
		if s.Name != want[i].Name {
			t.Errorf("skill[%d].Name: got %q, want %q", i, s.Name, want[i].Name)
		}
		if s.Version != want[i].Version {
			t.Errorf("skill[%d].Version: got %q, want %q", i, s.Version, want[i].Version)
		}
		if s.Description != want[i].Description {
			t.Errorf("skill[%d].Description: got %q, want %q", i, s.Description, want[i].Description)
		}
	}
}

// --------------------------------------------------------------------
// TestHealth
// --------------------------------------------------------------------

func TestHealth(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("expected /health, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "")
		if err := client.Health(context.Background()); err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"unavailable","message":"database down"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "")
		err := client.Health(context.Background())
		if err == nil {
			t.Fatal("expected error for unhealthy server")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", apiErr.StatusCode)
		}
		if apiErr.Message != "database down" {
			t.Errorf("expected message %q, got %q", "database down", apiErr.Message)
		}
	})
}

// --------------------------------------------------------------------
// TestGetExecution
// --------------------------------------------------------------------

func TestGetExecution(t *testing.T) {
	want := RunResult{
		ExecutionID: "exec-get-789",
		Status:      "completed",
		Output:      json.RawMessage(`{"ok": true}`),
		DurationMs:  300,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/executions/exec-get-789" {
			t.Errorf("expected path /v1/executions/exec-get-789, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	result, err := client.GetExecution(context.Background(), "exec-get-789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionID != want.ExecutionID {
		t.Errorf("ExecutionID: got %q, want %q", result.ExecutionID, want.ExecutionID)
	}
	if result.Status != want.Status {
		t.Errorf("Status: got %q, want %q", result.Status, want.Status)
	}
}

// --------------------------------------------------------------------
// TestGetExecutionLogs
// --------------------------------------------------------------------

func TestGetExecutionLogs(t *testing.T) {
	const wantLogs = "step 1: loading data\nstep 2: processing\ndone."

	t.Run("json_envelope", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/executions/exec-logs-1/logs" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"logs": wantLogs})
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		logs, err := client.GetExecutionLogs(context.Background(), "exec-logs-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logs != wantLogs {
			t.Errorf("got %q, want %q", logs, wantLogs)
		}
	})

	t.Run("plain_text", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, wantLogs)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		logs, err := client.GetExecutionLogs(context.Background(), "exec-logs-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logs != wantLogs {
			t.Errorf("got %q, want %q", logs, wantLogs)
		}
	})
}

// --------------------------------------------------------------------
// TestDownloadFiles
// --------------------------------------------------------------------

func TestDownloadFiles(t *testing.T) {
	t.Run("no_files", func(t *testing.T) {
		client := New("http://unused", "sk-test")
		result := &RunResult{Status: "completed"}
		if err := client.DownloadFiles(context.Background(), result, t.TempDir()); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})

	t.Run("extract_tar_gz", func(t *testing.T) {
		// Build a tar.gz archive in memory containing two files.
		archive := buildTestTarGz(t, map[string]string{
			"output/result.csv":  "a,b,c\n1,2,3\n",
			"output/summary.txt": "all good",
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		client := New("http://unused", "sk-test")
		result := &RunResult{
			FilesURL:  srv.URL + "/files.tar.gz",
			FilesList: []string{"output/result.csv", "output/summary.txt"},
		}

		destDir := t.TempDir()
		if err := client.DownloadFiles(context.Background(), result, destDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify extracted files.
		assertFileContent(t, filepath.Join(destDir, "output", "result.csv"), "a,b,c\n1,2,3\n")
		assertFileContent(t, filepath.Join(destDir, "output", "summary.txt"), "all good")
	})

	t.Run("path_traversal_rejected", func(t *testing.T) {
		archive := buildTestTarGz(t, map[string]string{
			"../../etc/passwd": "root:x:0:0",
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		client := New("http://unused", "sk-test")
		result := &RunResult{FilesURL: srv.URL + "/evil.tar.gz"}

		err := client.DownloadFiles(context.Background(), result, t.TempDir())
		if err == nil {
			t.Fatal("expected path traversal error, got nil")
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Fatalf("expected path traversal error, got: %v", err)
		}
	})
}

// --------------------------------------------------------------------
// TestAPIError
// --------------------------------------------------------------------

func TestAPIError(t *testing.T) {
	t.Run("structured_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"error":"invalid_request","message":"skill field is required"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		_, err := client.Run(context.Background(), RunRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("expected status 422, got %d", apiErr.StatusCode)
		}
		if apiErr.ErrorCode != "invalid_request" {
			t.Errorf("expected error code %q, got %q", "invalid_request", apiErr.ErrorCode)
		}
		if apiErr.Message != "skill field is required" {
			t.Errorf("expected message %q, got %q", "skill field is required", apiErr.Message)
		}

		wantMsg := "skillbox: 422 invalid_request: skill field is required"
		if apiErr.Error() != wantMsg {
			t.Errorf("Error() = %q, want %q", apiErr.Error(), wantMsg)
		}
	})

	t.Run("unstructured_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error")
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		_, err := client.ListSkills(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", apiErr.StatusCode)
		}
		if apiErr.Message != "internal server error" {
			t.Errorf("expected message %q, got %q", "internal server error", apiErr.Message)
		}
	})

	t.Run("error_code_only", func(t *testing.T) {
		e := &APIError{StatusCode: 401, ErrorCode: "unauthorized"}
		want := "skillbox: 401 unauthorized"
		if e.Error() != want {
			t.Errorf("Error() = %q, want %q", e.Error(), want)
		}
	})

	t.Run("status_only", func(t *testing.T) {
		e := &APIError{StatusCode: 500}
		want := "skillbox: 500"
		if e.Error() != want {
			t.Errorf("Error() = %q, want %q", e.Error(), want)
		}
	})
}

// --------------------------------------------------------------------
// TestRegisterSkill
// --------------------------------------------------------------------

func TestRegisterSkill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/skills" {
			t.Errorf("expected path /v1/skills, got %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart content-type, got %q", ct)
		}

		// Parse the multipart form.
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("get form file: %v", err)
		}
		defer file.Close() //nolint:errcheck

		if header.Filename != "test-skill.zip" {
			t.Errorf("expected filename test-skill.zip, got %q", header.Filename)
		}

		data, _ := io.ReadAll(file)
		if string(data) != "fake-zip-content" {
			t.Errorf("unexpected file content: %q", string(data))
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Create a temporary zip file.
	tmpFile := filepath.Join(t.TempDir(), "test-skill.zip")
	if err := os.WriteFile(tmpFile, []byte("fake-zip-content"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	client := New(srv.URL, "sk-test")
	if err := client.RegisterSkill(context.Background(), tmpFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --------------------------------------------------------------------
// TestListFiles
// --------------------------------------------------------------------

func TestListFiles(t *testing.T) {
	parentID := "parent-abc"
	want := []FileInfo{
		{
			ID:          "file-1",
			TenantID:    "tenant-1",
			SessionID:   "sess-1",
			ExecutionID: "exec-1",
			Name:        "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   12345,
			S3Key:       "tenant/exec/v1/report.pdf",
			Version:     1,
			ParentID:    nil,
			CreatedAt:   "2024-01-01T00:00:00Z",
			UpdatedAt:   "2024-01-01T00:00:00Z",
		},
		{
			ID:          "file-2",
			TenantID:    "tenant-1",
			SessionID:   "sess-1",
			ExecutionID: "exec-1",
			Name:        "data.csv",
			ContentType: "text/csv",
			SizeBytes:   678,
			S3Key:       "tenant/exec/v1/data.csv",
			Version:     2,
			ParentID:    &parentID,
			CreatedAt:   "2024-01-02T00:00:00Z",
			UpdatedAt:   "2024-01-02T00:00:00Z",
		},
	}

	t.Run("all_filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/v1/files" {
				t.Errorf("expected path /v1/files, got %s", r.URL.Path)
			}
			q := r.URL.Query()
			if got := q.Get("session_id"); got != "sess-1" {
				t.Errorf("expected session_id=sess-1, got %q", got)
			}
			if got := q.Get("execution_id"); got != "exec-1" {
				t.Errorf("expected execution_id=exec-1, got %q", got)
			}
			if got := q.Get("limit"); got != "10" {
				t.Errorf("expected limit=10, got %q", got)
			}
			if got := q.Get("offset"); got != "5" {
				t.Errorf("expected offset=5, got %q", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
				t.Errorf("expected Bearer sk-test, got %q", got)
			}
			if got := r.Header.Get("X-Tenant-ID"); got != "tenant-1" {
				t.Errorf("expected X-Tenant-ID tenant-1, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(want)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test", WithTenant("tenant-1"))
		files, err := client.ListFiles(context.Background(), FileFilter{
			SessionID:   "sess-1",
			ExecutionID: "exec-1",
			Limit:       10,
			Offset:      5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(files))
		}
		if files[0].Name != "report.pdf" {
			t.Errorf("files[0].Name: got %q, want %q", files[0].Name, "report.pdf")
		}
		if files[1].Version != 2 {
			t.Errorf("files[1].Version: got %d, want 2", files[1].Version)
		}
		if files[1].ParentID == nil || *files[1].ParentID != "parent-abc" {
			t.Errorf("files[1].ParentID: expected %q", "parent-abc")
		}
	})

	t.Run("no_filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				t.Errorf("expected no query params, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]FileInfo{})
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		files, err := client.ListFiles(context.Background(), FileFilter{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("expected 0 files, got %d", len(files))
		}
	})
}

// --------------------------------------------------------------------
// TestGetFile
// --------------------------------------------------------------------

func TestGetFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := FileInfo{
			ID:          "file-abc-123",
			TenantID:    "tenant-1",
			Name:        "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   12345,
			S3Key:       "tenant/exec/v1/report.pdf",
			Version:     1,
			CreatedAt:   "2024-01-01T00:00:00Z",
			UpdatedAt:   "2024-01-01T00:00:00Z",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/v1/files/file-abc-123" {
				t.Errorf("expected path /v1/files/file-abc-123, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(want)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		file, err := client.GetFile(context.Background(), "file-abc-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file.ID != want.ID {
			t.Errorf("ID: got %q, want %q", file.ID, want.ID)
		}
		if file.Name != want.Name {
			t.Errorf("Name: got %q, want %q", file.Name, want.Name)
		}
		if file.SizeBytes != want.SizeBytes {
			t.Errorf("SizeBytes: got %d, want %d", file.SizeBytes, want.SizeBytes)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"not_found","message":"file not found"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		_, err := client.GetFile(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for not found file")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.StatusCode)
		}
		if apiErr.ErrorCode != "not_found" {
			t.Errorf("expected error code %q, got %q", "not_found", apiErr.ErrorCode)
		}
	})
}

// --------------------------------------------------------------------
// TestDownloadFile
// --------------------------------------------------------------------

func TestDownloadFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wantContent := "Hello, this is the file content!\nLine two."

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/v1/files/file-dl-1/download" {
				t.Errorf("expected path /v1/files/file-dl-1/download, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprint(w, wantContent)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		destPath := filepath.Join(t.TempDir(), "downloaded.txt")
		if err := client.DownloadFile(context.Background(), "file-dl-1", destPath); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertFileContent(t, destPath, wantContent)
	})

	t.Run("creates_parent_dirs", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprint(w, "nested content")
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		destPath := filepath.Join(t.TempDir(), "sub", "dir", "file.txt")
		if err := client.DownloadFile(context.Background(), "file-dl-2", destPath); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertFileContent(t, destPath, "nested content")
	})

	t.Run("server_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"internal","message":"storage unavailable"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		destPath := filepath.Join(t.TempDir(), "wont-exist.txt")
		err := client.DownloadFile(context.Background(), "file-dl-3", destPath)
		if err == nil {
			t.Fatal("expected error for server failure")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", apiErr.StatusCode)
		}
	})
}

// --------------------------------------------------------------------
// TestUpdateFile
// --------------------------------------------------------------------

func TestUpdateFile(t *testing.T) {
	want := FileInfo{
		ID:          "file-upd-1",
		TenantID:    "tenant-1",
		Name:        "report-v2.pdf",
		ContentType: "application/pdf",
		SizeBytes:   9999,
		S3Key:       "tenant/exec/v2/report.pdf",
		Version:     2,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-02T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file-upd-1" {
			t.Errorf("expected path /v1/files/file-upd-1, got %s", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart content-type, got %q", ct)
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("get form file: %v", err)
		}
		defer file.Close() //nolint:errcheck

		if header.Filename != "update-content.bin" {
			t.Errorf("expected filename update-content.bin, got %q", header.Filename)
		}

		data, _ := io.ReadAll(file)
		if string(data) != "updated-file-bytes" {
			t.Errorf("unexpected file content: %q", string(data))
		}

		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("expected Bearer sk-test, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "update-content.bin")
	if err := os.WriteFile(tmpFile, []byte("updated-file-bytes"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	client := New(srv.URL, "sk-test", WithTenant("tenant-1"))
	result, err := client.UpdateFile(context.Background(), "file-upd-1", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != want.ID {
		t.Errorf("ID: got %q, want %q", result.ID, want.ID)
	}
	if result.Version != 2 {
		t.Errorf("Version: got %d, want 2", result.Version)
	}
	if result.Name != want.Name {
		t.Errorf("Name: got %q, want %q", result.Name, want.Name)
	}
}

// --------------------------------------------------------------------
// TestDeleteFile
// --------------------------------------------------------------------

func TestDeleteFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			if r.URL.Path != "/v1/files/file-del-1" {
				t.Errorf("expected path /v1/files/file-del-1, got %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
				t.Errorf("expected Bearer sk-test, got %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		if err := client.DeleteFile(context.Background(), "file-del-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"not_found","message":"file not found"}`)
		}))
		defer srv.Close()

		client := New(srv.URL, "sk-test")
		err := client.DeleteFile(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for not found file")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.StatusCode)
		}
		if apiErr.ErrorCode != "not_found" {
			t.Errorf("expected error code %q, got %q", "not_found", apiErr.ErrorCode)
		}
	})
}

// --------------------------------------------------------------------
// TestListFileVersions
// --------------------------------------------------------------------

func TestListFileVersions(t *testing.T) {
	parentID := "file-ver-1"
	want := []FileInfo{
		{
			ID:          "file-ver-1",
			TenantID:    "tenant-1",
			Name:        "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   10000,
			S3Key:       "tenant/exec/v1/report.pdf",
			Version:     1,
			CreatedAt:   "2024-01-01T00:00:00Z",
			UpdatedAt:   "2024-01-01T00:00:00Z",
		},
		{
			ID:          "file-ver-1-v2",
			TenantID:    "tenant-1",
			Name:        "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   12000,
			S3Key:       "tenant/exec/v2/report.pdf",
			Version:     2,
			ParentID:    &parentID,
			CreatedAt:   "2024-01-02T00:00:00Z",
			UpdatedAt:   "2024-01-02T00:00:00Z",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file-ver-1/versions" {
			t.Errorf("expected path /v1/files/file-ver-1/versions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("expected Bearer sk-test, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	versions, err := client.ListFileVersions(context.Background(), "file-ver-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 1 {
		t.Errorf("versions[0].Version: got %d, want 1", versions[0].Version)
	}
	if versions[1].Version != 2 {
		t.Errorf("versions[1].Version: got %d, want 2", versions[1].Version)
	}
	if versions[1].ParentID == nil || *versions[1].ParentID != "file-ver-1" {
		t.Errorf("versions[1].ParentID: expected %q", "file-ver-1")
	}
	if versions[1].SizeBytes != 12000 {
		t.Errorf("versions[1].SizeBytes: got %d, want 12000", versions[1].SizeBytes)
	}
}

// --------------------------------------------------------------------
// TestClient_UploadFileFromReader
// --------------------------------------------------------------------

func TestClient_UploadFileFromReader(t *testing.T) {
	want := FileInfo{
		ID:          "test-file-123",
		TenantID:    "t1",
		Name:        "test.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		SizeBytes:   1024,
		S3Key:       "t1/uploads/xxx/test.xlsx",
		Version:     1,
	}

	testContent := []byte("fake-xlsx-content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Errorf("expected path /v1/files, got %s", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected Content-Type to start with multipart/form-data, got %q", ct)
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("get form file: %v", err)
		}
		defer file.Close() //nolint:errcheck

		if header.Filename != "test.xlsx" {
			t.Errorf("expected filename test.xlsx, got %q", header.Filename)
		}

		data, _ := io.ReadAll(file)
		if string(data) != string(testContent) {
			t.Errorf("unexpected file content: %q", string(data))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	result, err := client.UploadFileFromReader(context.Background(), "test.xlsx", bytes.NewReader(testContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != want.ID {
		t.Errorf("ID: got %q, want %q", result.ID, want.ID)
	}
	if result.Name != want.Name {
		t.Errorf("Name: got %q, want %q", result.Name, want.Name)
	}
}

func TestClient_UploadFileFromReader_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal","message":"storage unavailable"}`)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test")
	_, err := client.UploadFileFromReader(context.Background(), "test.xlsx", bytes.NewReader([]byte("content")))
	if err == nil {
		t.Fatal("expected error for server failure, got nil")
	}
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

// buildTestTarGz creates an in-memory tar.gz archive from a map of
// path -> content pairs.
func buildTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("write tar header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content for %s: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// assertFileContent reads the file at path and asserts its content matches want.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got := string(data); got != want {
		t.Errorf("%s: got %q, want %q", path, got, want)
	}
}
