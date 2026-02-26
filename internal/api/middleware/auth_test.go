package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestAuthMiddleware_MissingHeader verifies that a request without an
// Authorization header is rejected with 401.
func TestAuthMiddleware_MissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header set.

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body response.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error != "unauthorized" {
		t.Errorf("error code = %q, want %q", body.Error, "unauthorized")
	}
	if body.Message != "missing Authorization header" {
		t.Errorf("message = %q, want %q", body.Message, "missing Authorization header")
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted")
	}
}

// TestAuthMiddleware_InvalidFormat verifies that a non-Bearer scheme is rejected.
func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body response.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error != "unauthorized" {
		t.Errorf("error code = %q, want %q", body.Error, "unauthorized")
	}
	if body.Message != "Authorization header must use Bearer scheme" {
		t.Errorf("message = %q, want %q", body.Message, "Authorization header must use Bearer scheme")
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted")
	}
}

// TestAuthMiddleware_EmptyBearer verifies that "Bearer " with an empty token
// is rejected.
func TestAuthMiddleware_EmptyBearer(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer ")

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body response.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error != "unauthorized" {
		t.Errorf("error code = %q, want %q", body.Error, "unauthorized")
	}
	if body.Message != "empty bearer token" {
		t.Errorf("message = %q, want %q", body.Message, "empty bearer token")
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted")
	}
}

// TestAuthMiddleware_EmptyBearerWithSpaces verifies that "Bearer    " (only
// whitespace after Bearer) is also rejected.
func TestAuthMiddleware_EmptyBearerWithSpaces(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer    ")

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted")
	}
}

// Note: Testing a valid Bearer token that reaches the store lookup would
// require either a mock store or a real database. The AuthMiddleware accepts
// a *store.Store (concrete type, not an interface), so we cannot easily mock
// GetAPIKeyByHash without a database connection. Those tests are deferred
// until the store is refactored to use an interface.
