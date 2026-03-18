package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	skillboxsdk "github.com/devs-group/skillbox/sdks/go"
)

// TestE2E_CreateFromFields_SDK_To_Handler tests the full stack:
// SDK client → HTTP → Gin router → auth middleware → CreateFromFields handler → validation.
func TestE2E_CreateFromFields_SDK_To_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	srv := httptest.NewServer(router)
	defer srv.Close()

	client := skillboxsdk.New(srv.URL, testToken, skillboxsdk.WithTenant(testTenantID))

	t.Run("rejects_empty_name", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Description: "test", Code: "print('hello')",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		assertAPIErrorStatus(t, err, http.StatusBadRequest)
	})

	t.Run("rejects_missing_description", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "test-skill", Code: "print('hello')",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		assertAPIErrorStatus(t, err, http.StatusBadRequest)
	})

	t.Run("rejects_missing_code", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "test-skill", Description: "test",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		assertAPIErrorStatus(t, err, http.StatusBadRequest)
	})

	t.Run("rejects_path_traversal_name", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "../etc/passwd", Description: "evil", Code: "import os",
		})
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
		assertAPIErrorStatus(t, err, http.StatusBadRequest)
	})

	t.Run("rejects_invalid_version", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "test", Description: "test", Code: "print()", Version: "not-semver",
		})
		if err == nil {
			t.Fatal("expected error for invalid version")
		}
		assertAPIErrorStatus(t, err, http.StatusBadRequest)
	})

	t.Run("valid_request_passes_validation", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "my-valid-skill", Description: "A valid skill",
			Code: "print('hello')", Lang: "python", Version: "1.0.0",
		})
		// Will fail at nil registry (500), but NOT at validation (400)
		if err == nil {
			t.Fatal("expected error from nil registry")
		}
		// The key assertion: the status is NOT 400
		assertAPIErrorStatusNot(t, err, http.StatusBadRequest)
		assertAPIErrorStatusNot(t, err, http.StatusNotFound)
	})

	t.Run("defaults_applied_for_missing_lang_and_version", func(t *testing.T) {
		expectAuthLookup(mock, testToken)
		_, err := client.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "defaults-test", Description: "Testing defaults",
			Code: "print('hello')",
			// Lang and Version intentionally omitted
		})
		if err == nil {
			t.Fatal("expected error from nil registry")
		}
		// Should NOT be 400 — defaults should have been applied
		assertAPIErrorStatusNot(t, err, http.StatusBadRequest)
	})

	t.Run("auth_rejects_bad_token", func(t *testing.T) {
		expectAuthLookupNotFound(mock, "sk-bad-token")
		badClient := skillboxsdk.New(srv.URL, "sk-bad-token")
		_, err := badClient.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "test", Description: "test", Code: "print()",
		})
		if err == nil {
			t.Fatal("expected error for bad auth")
		}
		assertAPIErrorStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("no_auth_header_rejected", func(t *testing.T) {
		noAuthClient := skillboxsdk.New(srv.URL, "")
		_, err := noAuthClient.UpsertSkillFromFields(context.Background(), skillboxsdk.CreateFromFieldsRequest{
			Name: "test", Description: "test", Code: "print()",
		})
		if err == nil {
			t.Fatal("expected error for missing auth")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// assertAPIErrorStatus checks that the error contains the given HTTP status code.
func assertAPIErrorStatus(t *testing.T, err error, wantStatus int) {
	t.Helper()
	msg := err.Error()
	wantStr := strings.Replace(http.StatusText(wantStatus), " ", "", -1)
	statusStr := ""
	switch wantStatus {
	case 400:
		statusStr = "400"
	case 401:
		statusStr = "401"
	case 404:
		statusStr = "404"
	case 500:
		statusStr = "500"
	}
	if !strings.Contains(msg, statusStr) && !strings.Contains(msg, wantStr) {
		t.Errorf("expected status %d in error, got: %s", wantStatus, msg)
	}
}

// assertAPIErrorStatusNot checks that the error does NOT contain the given HTTP status code.
func assertAPIErrorStatusNot(t *testing.T, err error, notStatus int) {
	t.Helper()
	msg := err.Error()
	statusStr := ""
	switch notStatus {
	case 400:
		statusStr = " 400 "
	case 404:
		statusStr = " 404 "
	}
	if strings.Contains(msg, statusStr) {
		t.Errorf("should NOT contain status %d, but got: %s", notStatus, msg)
	}
}
