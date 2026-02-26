package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealth_ReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	// Health does not use the store, so passing nil is safe.
	handler := Health(nil)
	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("body[\"status\"] = %q, want %q", body["status"], "ok")
	}
}

// TODO: TestReady_ReturnsReady and TestReady_ReturnsNotReady require a mock
// store that implements Ping(). The Ready handler calls s.Ping() which needs
// a real database connection. Implementing these tests requires either:
// - An interface-based store mock, or
// - An in-memory database driver.
// Skipping for now until the store is refactored to accept an interface.
