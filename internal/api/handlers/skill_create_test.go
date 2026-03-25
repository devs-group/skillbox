package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/config"
)

func TestCreateFromFields_ValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The handler requires reg + store for the actual upload, but validation
	// errors are returned before those are called. We can pass nil safely
	// for tests that only exercise request validation.
	cfg := &config.Config{MaxSkillSize: 10 << 20} // 10MB
	handler := CreateFromFields(nil, nil, cfg, nil)

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty body",
			body:       map[string]any{},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "missing name",
			body:       map[string]any{"description": "test", "code": "print()"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "missing description",
			body:       map[string]any{"name": "test", "code": "print()"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "missing code",
			body:       map[string]any{"name": "test", "description": "test"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "invalid skill name with path traversal",
			body:       map[string]any{"name": "../evil", "description": "test", "code": "print()"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "invalid skill name with slashes",
			body:       map[string]any{"name": "foo/bar", "description": "test", "code": "print()"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
		{
			name:       "invalid version",
			body:       map[string]any{"name": "test", "description": "test", "code": "print()", "version": "abc"},
			wantStatus: http.StatusBadRequest,
			wantError:  "bad_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyJSON, _ := json.Marshal(tt.body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/skills/from-fields", bytes.NewReader(bodyJSON))
			c.Request.Header.Set("Content-Type", "application/json")

			// Set tenant ID in context (normally set by middleware)
			c.Set("tenant_id", "test-tenant")

			handler(c)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if errCode, ok := resp["error"].(string); !ok || errCode != tt.wantError {
				t.Errorf("error = %v, want %q", resp["error"], tt.wantError)
			}
		})
	}
}

func TestCreateFromFields_DefaultLangAndVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test verifies that the handler correctly defaults lang and version.
	// We can't complete the full flow without a registry, but we can verify
	// the handler gets past validation with minimal fields.
	// The handler will fail at reg.Upload (nil pointer), which confirms
	// validation passed. We catch the panic to verify.

	cfg := &config.Config{MaxSkillSize: 10 << 20}
	handler := CreateFromFields(nil, nil, cfg, nil)

	body := map[string]any{
		"name":        "test-skill",
		"description": "A test skill",
		"code":        "print('hello')",
		// lang and version intentionally omitted
	}
	bodyJSON, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/skills/from-fields", bytes.NewReader(bodyJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("tenant_id", "test-tenant")

	// The handler will panic or error when it tries to use the nil registry.
	// That's expected — we just want to confirm it got past validation.
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: nil pointer on reg.Upload
				t.Logf("recovered expected panic (nil registry): %v", r)
			}
		}()
		handler(c)
	}()

	// If we got a 400, validation rejected it (bad)
	if w.Code == http.StatusBadRequest {
		t.Errorf("request with default lang/version should pass validation, got 400: %s", w.Body.String())
	}
}
