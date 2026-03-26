package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/handlers"
	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/store"
)

// apiKeyColumns mirrors the SELECT column order in store.GetAPIKeyByHash.
var apiKeyColumns = []string{
	"id", "key_hash", "tenant_id", "name", "created_at", "revoked_at",
}

// fileColumns mirrors the SELECT column order in store.GetFile / ListFiles.
var fileColumns = []string{
	"id", "tenant_id", "session_id", "execution_id", "name", "content_type",
	"size_bytes", "s3_key", "version", "parent_id", "created_at", "updated_at",
}

// testToken is the raw API key token used across tests.
const testToken = "sk-test-key-0123456789abcdef"

// testTenantID is the tenant ID associated with the test API key.
const testTenantID = "tenant-abc"

// testKeyID is the database ID for the test API key.
const testKeyID = "key-uuid-1"

func init() {
	gin.SetMode(gin.TestMode)
}

// tokenHash returns the SHA-256 hex hash of the given token, matching the
// algorithm used by AuthMiddleware.
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// testConfig returns a minimal *config.Config sufficient for NewRouter. The
// router itself does not read most config fields, but the function signature
// requires a non-nil config.
func testConfig() *config.Config {
	return &config.Config{
		APIPort: "8080",
	}
}

// setupRouter creates a *gin.Engine wired through NewRouter with a sqlmock-
// backed store and a nil collector. The caller must call cleanup() when done
// and verify mock expectations. MonitorPingsOption is enabled so readiness
// probes can be tested via ExpectPing.
func setupRouter(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	st := store.NewWithDB(db)
	cfg := testConfig()

	// Pass nil runner and nil registry since we are not testing execution
	// or skill endpoints. Pass nil collector as well; the router skips
	// file route registration when no collector is provided.
	router := NewRouter(cfg, st, nil, nil, nil, nil, nil, nil)

	return router, mock, func() { db.Close() } //nolint:errcheck
}


// expectAuthLookup sets up the sqlmock expectation for an AuthMiddleware
// key lookup. It expects a query against sandbox.api_keys matching the
// SHA-256 hash of the given token, and returns a valid (non-revoked) key
// for testTenantID.
func expectAuthLookup(mock sqlmock.Sqlmock, token string) {
	hash := tokenHash(token)
	now := time.Now()
	mock.ExpectQuery("SELECT id, key_hash, tenant_id, name, created_at, revoked_at").
		WithArgs(hash).
		WillReturnRows(sqlmock.NewRows(apiKeyColumns).
			AddRow(testKeyID, hash, testTenantID, "test-key", now, nil))
}

// expectAuthLookupRevoked sets up a mock expectation that returns a key
// whose revoked_at is non-nil.
func expectAuthLookupRevoked(mock sqlmock.Sqlmock, token string) {
	hash := tokenHash(token)
	now := time.Now()
	revoked := now.Add(-time.Hour)
	mock.ExpectQuery("SELECT id, key_hash, tenant_id, name, created_at, revoked_at").
		WithArgs(hash).
		WillReturnRows(sqlmock.NewRows(apiKeyColumns).
			AddRow(testKeyID, hash, testTenantID, "test-key", now, &revoked))
}

// expectAuthLookupNotFound sets up a mock expectation that returns zero
// rows, simulating an unknown API key.
func expectAuthLookupNotFound(mock sqlmock.Sqlmock, token string) {
	hash := tokenHash(token)
	mock.ExpectQuery("SELECT id, key_hash, tenant_id, name, created_at, revoked_at").
		WithArgs(hash).
		WillReturnRows(sqlmock.NewRows(apiKeyColumns))
}

// decodeError is a helper that decodes a response body into an APIError
// and fails the test if decoding fails.
func decodeError(t *testing.T, body []byte) response.APIError {
	t.Helper()
	var e response.APIError
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("failed to decode error response: %v\nbody: %s", err, string(body))
	}
	return e
}

// -----------------------------------------------------------------------
// Health endpoints
// -----------------------------------------------------------------------

func TestHealth_Returns200(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /health: status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, want %q", body["status"], "ok")
	}
}

func TestReady_Healthy_Returns200(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	// The Ready handler calls s.Ping which issues db.PingContext.
	mock.ExpectPing()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /ready: status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("status = %v, want %q", body["status"], "ready")
	}
	checks, ok := body["checks"].(map[string]interface{})
	if !ok {
		t.Fatalf("checks field missing or not an object")
	}
	if checks["postgres"] != "ok" {
		t.Errorf("checks.postgres = %v, want %q", checks["postgres"], "ok")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestReady_DBDown_Returns503(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	mock.ExpectPing().WillReturnError(http.ErrServerClosed)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /ready: status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Errorf("status = %v, want %q", body["status"], "not_ready")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestHealth_NoAuthRequired(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	// No Authorization header at all.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health without auth: status = %d, want %d", w.Code, http.StatusOK)
	}
}

// -----------------------------------------------------------------------
// Auth middleware (via router integration)
// -----------------------------------------------------------------------

func TestAuth_MissingAuthorizationHeader_Returns401(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	// Hit any v1 endpoint without Authorization header.
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "unauthorized" {
		t.Errorf("error = %q, want %q", e.Error, "unauthorized")
	}
	if e.Message != "missing Authorization header" {
		t.Errorf("message = %q, want %q", e.Message, "missing Authorization header")
	}
}

func TestAuth_InvalidScheme_Returns401(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "unauthorized" {
		t.Errorf("error = %q, want %q", e.Error, "unauthorized")
	}
}

func TestAuth_EmptyBearerToken_Returns401(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Bearer ")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "unauthorized" {
		t.Errorf("error = %q, want %q", e.Error, "unauthorized")
	}
	if e.Message != "empty bearer token" {
		t.Errorf("message = %q, want %q", e.Message, "empty bearer token")
	}
}

func TestAuth_UnknownAPIKey_Returns401(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	expectAuthLookupNotFound(mock, "sk-unknown-key")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Bearer sk-unknown-key")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "unauthorized" {
		t.Errorf("error = %q, want %q", e.Error, "unauthorized")
	}
	if e.Message != "invalid or revoked API key" {
		t.Errorf("message = %q, want %q", e.Message, "invalid or revoked API key")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAuth_RevokedAPIKey_Returns401(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	expectAuthLookupRevoked(mock, testToken)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "unauthorized" {
		t.Errorf("error = %q, want %q", e.Error, "unauthorized")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// Tenant middleware (via router integration)
// -----------------------------------------------------------------------

func TestTenant_MismatchedXTenantIDHeader_Returns403(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Tenant-ID", "different-tenant")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "forbidden" {
		t.Errorf("error = %q, want %q", e.Error, "forbidden")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTenant_MatchingXTenantIDHeader_Allowed(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	w := httptest.NewRecorder()
	// Hit GET /v1/executions/:id which will proceed past middleware. The
	// handler will attempt a DB query; we set up a "not found" response.
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/some-id", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Tenant-ID", testTenantID)
	router.ServeHTTP(w, req)

	// If we got past auth+tenant middleware, we should see either 200 or
	// a handler-level error (not 401 or 403).
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("should have passed middleware, but got status %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// File endpoints (via full router integration)
//
// Because NewRouter guards file routes behind `col[0] != nil`, we build
// the router manually using the same handlers and middleware so we can
// test the full request -> middleware -> handler -> response cycle.
// -----------------------------------------------------------------------

// buildFilesRouter creates a Gin engine with health, auth/tenant middleware,
// and the /v1/files routes, using the given store. The collector is nil
// which is fine for all endpoints except Download (which would panic on the
// nil collector call).
func buildFilesRouter(s *store.Store) *gin.Engine {
	// We import the sub-packages via the api package imports. Since this
	// test file is in package `api`, we use the same imports declared by
	// router.go.
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Health (no auth).
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := engine.Group("/v1")
	v1.Use(authMiddlewareForTest(s))
	v1.Use(tenantMiddlewareForTest())
	{
		fh := newFilesHandlerForTest(s)
		files := v1.Group("/files")
		{
			files.GET("", fh.list)
			files.GET("/:id", fh.get)
			files.GET("/:id/download", fh.download)
			files.DELETE("/:id", fh.del)
			files.GET("/:id/versions", fh.versions)
		}
	}

	return engine
}

// The test file is in package api, which imports middleware and handlers as
// sub-packages. We replicate thin wrappers that delegate to those packages
// so we do not need to duplicate their logic.

func authMiddlewareForTest(s *store.Store) gin.HandlerFunc {
	// Delegate to the real middleware.
	return middleware.AuthMiddleware(s, "http://localhost:4445")
}

func tenantMiddlewareForTest() gin.HandlerFunc {
	return middleware.TenantMiddleware()
}

// filesHandlerWrapper wraps handlers.FilesHandler methods for route binding.
type filesHandlerWrapper struct {
	inner *handlers.FilesHandler
}

func newFilesHandlerForTest(s *store.Store) *filesHandlerWrapper {
	return &filesHandlerWrapper{
		inner: handlers.NewFilesHandler(s, nil, 50*1024*1024),
	}
}

func (w *filesHandlerWrapper) list(c *gin.Context)     { w.inner.List(c) }
func (w *filesHandlerWrapper) get(c *gin.Context)      { w.inner.Get(c) }
func (w *filesHandlerWrapper) del(c *gin.Context)      { w.inner.Delete(c) }
func (w *filesHandlerWrapper) download(c *gin.Context)  { w.inner.Download(c) }
func (w *filesHandlerWrapper) versions(c *gin.Context)  { w.inner.Versions(c) }

// newFilesRouter is a convenience function that creates a sqlmock-backed
// store and a router with all file routes registered.
func newFilesRouter(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	s := store.NewWithDB(db)
	router := buildFilesRouter(s)
	return router, mock, func() { db.Close() } //nolint:errcheck
}

// authRequest creates an *http.Request with a valid Bearer token header.
func authRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}

// -----------------------------------------------------------------------
// GET /v1/files (List)
// -----------------------------------------------------------------------

func TestRouter_ListFiles_Returns200WithFiles(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	// Auth lookup.
	expectAuthLookup(mock, testToken)

	// ListFiles query.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs(testTenantID, "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("f-1", testTenantID, nil, nil, "data.csv", "text/csv",
				int64(512), "s3/key1", 1, nil, now, now).
			AddRow("f-2", testTenantID, "sess-1", nil, "output.json", "application/json",
				int64(1024), "s3/key2", 1, nil, now, now))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var files []store.File
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].ID != "f-1" {
		t.Errorf("files[0].ID = %q, want %q", files[0].ID, "f-1")
	}
	if files[1].Name != "output.json" {
		t.Errorf("files[1].Name = %q, want %q", files[1].Name, "output.json")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_ListFiles_EmptyReturnsArray(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs(testTenantID, "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Must be [] not null.
	if body := w.Body.String(); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_ListFiles_WithPaginationAndFilters(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs(testTenantID, "sess-42", "exec-99", 10, 20).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet,
		"/v1/files?session_id=sess-42&execution_id=exec-99&limit=10&offset=20"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_ListFiles_DBError_Returns500(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WillReturnError(http.ErrServerClosed)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "internal_error" {
		t.Errorf("error = %q, want %q", e.Error, "internal_error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_ListFiles_NoAuth_Returns401(t *testing.T) {
	router, _, cleanup := newFilesRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// -----------------------------------------------------------------------
// GET /v1/files/:id (Get)
// -----------------------------------------------------------------------

func TestRouter_GetFile_Returns200(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, "sess-1", "exec-1",
				"report.csv", "text/csv", int64(2048), "s3/report", 1, nil, now, now))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var f store.File
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.ID != "file-uuid-1" {
		t.Errorf("ID = %q, want %q", f.ID, "file-uuid-1")
	}
	if f.Name != "report.csv" {
		t.Errorf("Name = %q, want %q", f.Name, "report.csv")
	}
	if f.TenantID != testTenantID {
		t.Errorf("TenantID = %q, want %q", f.TenantID, testTenantID)
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want %d", f.Version, 1)
	}
	if f.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %d, want %d", f.SizeBytes, 2048)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_GetFile_NotFound_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/missing-uuid"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "not_found" {
		t.Errorf("error = %q, want %q", e.Error, "not_found")
	}
	if e.Message != "file not found" {
		t.Errorf("message = %q, want %q", e.Message, "file not found")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_GetFile_TenantIsolation_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	// SQL filters by tenant_id, so a file belonging to a different tenant
	// simply returns no rows for the caller's tenant.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "not_found" {
		t.Errorf("error = %q, want %q", e.Error, "not_found")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_GetFile_NoAuth_Returns401(t *testing.T) {
	router, _, cleanup := newFilesRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/file-uuid-1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// -----------------------------------------------------------------------
// DELETE /v1/files/:id
// -----------------------------------------------------------------------

func TestRouter_DeleteFile_Returns204(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	// GetFile to verify ownership.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "doomed.txt",
				"text/plain", int64(50), "s3/doomed", 1, nil, now, now))

	// DeleteFile.
	mock.ExpectExec("DELETE FROM sandbox.files").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodDelete, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, http.StatusNoContent, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_DeleteFile_NotFound_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodDelete, "/v1/files/missing-uuid"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_DeleteFile_TenantIsolation_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	// SQL filters by tenant_id, so a file belonging to a different tenant
	// simply returns no rows for the caller's tenant.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodDelete, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_DeleteFile_NoAuth_Returns401(t *testing.T) {
	router, _, cleanup := newFilesRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/files/file-uuid-1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRouter_DeleteFile_DBError_Returns500(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	// GetFile succeeds.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "doomed.txt",
				"text/plain", int64(50), "s3/doomed", 1, nil, now, now))

	// DeleteFile fails.
	mock.ExpectExec("DELETE FROM sandbox.files").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnError(http.ErrServerClosed)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodDelete, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "internal_error" {
		t.Errorf("error = %q, want %q", e.Error, "internal_error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// GET /v1/files/:id/versions
// -----------------------------------------------------------------------

func TestRouter_FileVersions_Returns200(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	parentID := "file-uuid-1"

	expectAuthLookup(mock, testToken)

	// GetFile (ownership check).
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "data.csv",
				"text/csv", int64(100), "s3/v1", 1, nil, now, now))

	// ListFileVersions (recursive CTE).
	mock.ExpectQuery("WITH RECURSIVE root AS").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-v2", testTenantID, nil, nil, "data.csv",
				"text/csv", int64(200), "s3/v2", 2, &parentID, now, now).
			AddRow("file-uuid-1", testTenantID, nil, nil, "data.csv",
				"text/csv", int64(100), "s3/v1", 1, nil, now, now))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/versions"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var versions []store.File
	if err := json.Unmarshal(w.Body.Bytes(), &versions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
	// Ordered by version descending.
	if versions[0].Version != 2 {
		t.Errorf("versions[0].Version = %d, want 2", versions[0].Version)
	}
	if versions[1].Version != 1 {
		t.Errorf("versions[1].Version = %d, want 1", versions[1].Version)
	}
	if versions[0].ParentID == nil || *versions[0].ParentID != "file-uuid-1" {
		t.Errorf("versions[0].ParentID = %v, want %q", versions[0].ParentID, "file-uuid-1")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_FileVersions_EmptyReturnsArray(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	// GetFile (ownership check).
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "data.csv",
				"text/csv", int64(100), "s3/v1", 1, nil, now, now))

	// ListFileVersions returns no rows (recursive CTE).
	mock.ExpectQuery("WITH RECURSIVE root AS").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/versions"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if body := w.Body.String(); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_FileVersions_NotFound_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/missing-uuid/versions"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "not_found" {
		t.Errorf("error = %q, want %q", e.Error, "not_found")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_FileVersions_TenantIsolation_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	// SQL filters by tenant_id, so a file belonging to a different tenant
	// simply returns no rows for the caller's tenant.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/versions"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_FileVersions_NoAuth_Returns401(t *testing.T) {
	router, _, cleanup := newFilesRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/file-uuid-1/versions", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRouter_FileVersions_DBError_Returns500(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	// GetFile succeeds.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "data.csv",
				"text/csv", int64(100), "s3/v1", 1, nil, now, now))

	// ListFileVersions fails (recursive CTE).
	mock.ExpectQuery("WITH RECURSIVE root AS").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnError(http.ErrServerClosed)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/versions"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	e := decodeError(t, w.Body.Bytes())
	if e.Error != "internal_error" {
		t.Errorf("error = %q, want %q", e.Error, "internal_error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// Error response format consistency
// -----------------------------------------------------------------------

func TestRouter_ErrorResponseFormat(t *testing.T) {
	// Every error response must have exactly "error" and "message" fields.
	router, _, cleanup := newFilesRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	router.ServeHTTP(w, req)

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := raw["error"]; !ok {
		t.Error("error response missing 'error' field")
	}
	if _, ok := raw["message"]; !ok {
		t.Error("error response missing 'message' field")
	}
}

// -----------------------------------------------------------------------
// Response headers
// -----------------------------------------------------------------------

func TestRouter_ResponseContentType(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs(testTenantID, "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files"))

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// Method not allowed / 404 for unregistered routes
// -----------------------------------------------------------------------

func TestRouter_UnknownRoute_Returns404(t *testing.T) {
	router, _, cleanup := setupRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	router.ServeHTTP(w, req)

	// Gin returns 404 for unmatched routes.
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// -----------------------------------------------------------------------
// GET /v1/files/:id/download (error path: nil collector)
// -----------------------------------------------------------------------

func TestRouter_DownloadFile_NilCollector_Returns500(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	// GetFile succeeds (needed before download attempt).
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, nil, nil, "report.pdf",
				"application/pdf", int64(4096), "s3/report.pdf", 1, nil, now, now))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/download"))

	// The handler will try to call h.collector.DownloadObject on a nil
	// collector. Gin's Recovery middleware catches the panic and returns 500.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (nil collector panic recovered)", w.Code, http.StatusInternalServerError)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_DownloadFile_NotFound_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/missing-uuid/download"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRouter_DownloadFile_TenantIsolation_Returns404(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	expectAuthLookup(mock, testToken)

	// SQL filters by tenant_id, so a file belonging to a different tenant
	// simply returns no rows for the caller's tenant.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1/download"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// File JSON serialization
// -----------------------------------------------------------------------

func TestRouter_GetFile_JSONStructure(t *testing.T) {
	router, mock, cleanup := newFilesRouter(t)
	defer cleanup()

	now := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	expectAuthLookup(mock, testToken)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", testTenantID).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", testTenantID, "sess-1", "exec-1",
				"result.json", "application/json", int64(512),
				"s3/result.json", 3, nil, now, now))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest(http.MethodGet, "/v1/files/file-uuid-1"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify all expected JSON fields are present.
	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	requiredFields := []string{
		"id", "tenant_id", "name", "content_type",
		"size_bytes", "s3_key", "version", "created_at", "updated_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required JSON field %q", field)
		}
	}

	// session_id and execution_id should be present when non-empty.
	if raw["session_id"] != "sess-1" {
		t.Errorf("session_id = %v, want %q", raw["session_id"], "sess-1")
	}
	if raw["execution_id"] != "exec-1" {
		t.Errorf("execution_id = %v, want %q", raw["execution_id"], "exec-1")
	}

	// parent_id should be omitted when nil (omitempty).
	if _, ok := raw["parent_id"]; ok {
		t.Errorf("parent_id should be omitted when nil, got %v", raw["parent_id"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// -----------------------------------------------------------------------
// Multiple auth scenarios combined
// -----------------------------------------------------------------------

func TestRouter_AuthScenarios(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantError  string
	}{
		{
			name:       "no header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "basic auth",
			authHeader: "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "bearer with empty token",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "bearer with whitespace only",
			authHeader: "Bearer   ",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "malformed header (no space)",
			authHeader: "Bearertoken123",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _, cleanup := newFilesRouter(t)
			defer cleanup()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			e := decodeError(t, w.Body.Bytes())
			if e.Error != tt.wantError {
				t.Errorf("error = %q, want %q", e.Error, tt.wantError)
			}
		})
	}
}

// TestRoute_SkillsFromFields_DoesNotConflict verifies that POST /v1/skills/from-fields
// is routed correctly and does not collide with POST /v1/skills (zip upload)
// or GET /v1/skills/:name/:version.
func TestRoute_SkillsFromFields_DoesNotConflict(t *testing.T) {
	router, mock, cleanup := setupRouter(t)
	defer cleanup()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // We expect the route to exist (not 404)
	}{
		{
			name:       "from-fields route exists",
			method:     http.MethodPost,
			path:       "/v1/skills/from-fields",
			wantStatus: http.StatusBadRequest, // No body → validation error, but NOT 404
		},
		{
			name:       "zip upload route still exists",
			method:     http.MethodPost,
			path:       "/v1/skills",
			wantStatus: http.StatusUnsupportedMediaType, // No content-type → 415, but NOT 404
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectAuthLookup(mock, testToken)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+testToken)
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Errorf("%s %s returned 404 — route not registered", tt.method, tt.path)
			}
			if w.Code != tt.wantStatus {
				t.Errorf("%s %s status = %d, want %d; body: %s", tt.method, tt.path, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
