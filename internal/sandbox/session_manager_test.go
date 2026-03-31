package sandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitExecDReady_ImmediateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cl := New(srv.URL, "test", srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := cl.WaitExecDReady(ctx, srv.URL); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestWaitExecDReady_SucceedsAfterRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			// Simulate ExecD not ready yet.
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cl := New(srv.URL, "test", srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cl.WaitExecDReady(ctx, srv.URL); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if n := calls.Load(); n < 3 {
		t.Fatalf("expected at least 3 ping attempts, got %d", n)
	}
}

func TestWaitExecDReady_TimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cl := New(srv.URL, "test", srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := cl.WaitExecDReady(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"connection refused", true},
		{"connection reset by peer", true},
		{"no such host", true},
		{"i/o timeout", true},
		{"Post \"http://host.docker.internal:41416/proxy/44772/files/upload\": EOF", true},
		{"some other error", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			var err error
			if tt.msg != "" {
				err = &testError{tt.msg}
			}
			if got := isConnectionError(err); got != tt.want {
				t.Errorf("isConnectionError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}

	// nil error
	if isConnectionError(nil) {
		t.Error("isConnectionError(nil) = true, want false")
	}
}

func TestIsSandboxGone(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{`opensandbox: DELETE /sandboxes/abc: unexpected status 404: {"code":"DOCKER::SANDBOX_NOT_FOUND"}`, true},
		{"status 404", true},
		{"SANDBOX_NOT_FOUND", true},
		{"status 500: internal error", false},
		{"connection refused", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := &testError{tt.msg}
			if got := isSandboxGone(err); got != tt.want {
				t.Errorf("isSandboxGone(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}

	if isSandboxGone(nil) {
		t.Error("isSandboxGone(nil) = true, want false")
	}
}

type testError struct{ s string }

func (e *testError) Error() string { return e.s }
