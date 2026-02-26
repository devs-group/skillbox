package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
)

// TestTenantMiddleware_SetsFromContext verifies that the tenant ID set by
// AuthMiddleware is available through the context after TenantMiddleware runs.
func TestTenantMiddleware_SetsFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// Simulate AuthMiddleware having set the tenant ID.
	c.Set(ContextKeyTenantID, "tenant-abc-123")

	var capturedTenant string
	handler := TenantMiddleware()
	handler(c)

	// Verify the tenant ID is still accessible after middleware ran.
	val, exists := c.Get(ContextKeyTenantID)
	if !exists {
		t.Fatal("expected tenant_id to exist in context after TenantMiddleware")
	}
	capturedTenant = val.(string)
	if capturedTenant != "tenant-abc-123" {
		t.Errorf("tenant_id = %q, want %q", capturedTenant, "tenant-abc-123")
	}

	// Should not be aborted.
	if c.IsAborted() {
		t.Error("context should not be aborted when tenant matches")
	}
}

// TestTenantMiddleware_MatchingHeader verifies that the middleware passes when
// X-Tenant-ID header matches the context tenant.
func TestTenantMiddleware_MatchingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("X-Tenant-ID", "tenant-abc-123")

	c.Set(ContextKeyTenantID, "tenant-abc-123")

	handler := TenantMiddleware()
	handler(c)

	if c.IsAborted() {
		t.Error("context should not be aborted when X-Tenant-ID matches")
	}

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestTenantMiddleware_MismatchedHeader verifies that an X-Tenant-ID header
// differing from the context tenant results in a 403 Forbidden.
func TestTenantMiddleware_MismatchedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("X-Tenant-ID", "tenant-evil-999")

	// AuthMiddleware would have set the real tenant.
	c.Set(ContextKeyTenantID, "tenant-abc-123")

	handler := TenantMiddleware()
	handler(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var body response.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error != "forbidden" {
		t.Errorf("error code = %q, want %q", body.Error, "forbidden")
	}
	if body.Message != "X-Tenant-ID header does not match the API key's tenant" {
		t.Errorf("message = %q, want %q", body.Message, "X-Tenant-ID header does not match the API key's tenant")
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted on tenant mismatch")
	}
}

// TestTenantMiddleware_MissingContextTenant verifies that the middleware
// returns 500 if tenant_id was never set in the context (programming error).
func TestTenantMiddleware_MissingContextTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// Do NOT set ContextKeyTenantID.

	handler := TenantMiddleware()
	handler(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	if !c.IsAborted() {
		t.Error("expected context to be aborted when tenant_id is missing")
	}
}

// TestGetTenantID verifies the convenience helper.
func TestGetTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ContextKeyTenantID, "tenant-xyz")

	got := GetTenantID(c)
	if got != "tenant-xyz" {
		t.Errorf("GetTenantID() = %q, want %q", got, "tenant-xyz")
	}
}

// TestGetTenantID_Panics verifies the convenience helper panics when tenant_id
// is not in context (indicating a programming error).
func TestGetTenantID_Panics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected GetTenantID to panic when tenant_id is missing")
		}
	}()

	GetTenantID(c)
}
