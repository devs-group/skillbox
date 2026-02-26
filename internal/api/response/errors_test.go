package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	RespondError(c, http.StatusNotFound, "not_found", "the requested resource was not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var body APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error != "not_found" {
		t.Errorf("Error = %q, want %q", body.Error, "not_found")
	}
	if body.Message != "the requested resource was not found" {
		t.Errorf("Message = %q, want %q", body.Message, "the requested resource was not found")
	}
	if body.Details != nil {
		t.Errorf("Details = %v, want nil", body.Details)
	}

	// Verify Content-Type header.
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}
}

func TestRespondError_DifferentCodes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
	}{
		{"bad request", http.StatusBadRequest, "bad_request", "invalid input"},
		{"unauthorized", http.StatusUnauthorized, "unauthorized", "missing token"},
		{"forbidden", http.StatusForbidden, "forbidden", "access denied"},
		{"internal error", http.StatusInternalServerError, "internal_error", "something went wrong"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			RespondError(c, tt.status, tt.code, tt.message)

			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}

			var body APIError
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body.Error != tt.code {
				t.Errorf("Error = %q, want %q", body.Error, tt.code)
			}
			if body.Message != tt.message {
				t.Errorf("Message = %q, want %q", body.Message, tt.message)
			}
		})
	}
}

func TestRespondErrorWithDetails(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	details := map[string]string{
		"field": "name",
		"issue": "must not be empty",
	}
	RespondErrorWithDetails(c, http.StatusUnprocessableEntity, "validation_error", "invalid request body", details)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["error"] != "validation_error" {
		t.Errorf("error = %q, want %q", body["error"], "validation_error")
	}
	if body["message"] != "invalid request body" {
		t.Errorf("message = %q, want %q", body["message"], "invalid request body")
	}

	// Details should be present.
	detailsRaw, ok := body["details"]
	if !ok {
		t.Fatal("expected details field to be present")
	}
	detailsMap, ok := detailsRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("details should be a map, got %T", detailsRaw)
	}
	if detailsMap["field"] != "name" {
		t.Errorf("details[\"field\"] = %q, want %q", detailsMap["field"], "name")
	}
	if detailsMap["issue"] != "must not be empty" {
		t.Errorf("details[\"issue\"] = %q, want %q", detailsMap["issue"], "must not be empty")
	}
}

func TestRespondError_OmitsNilDetails(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	RespondError(c, http.StatusBadRequest, "bad_request", "something is wrong")

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The "details" field should be absent (omitempty).
	if _, exists := raw["details"]; exists {
		t.Error("expected 'details' key to be omitted when nil")
	}
}
