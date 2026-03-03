package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/store"
)

// fileColumns matches the SELECT column order in store.GetFile / ListFiles.
var handlerFileColumns = []string{
	"id", "tenant_id", "session_id", "execution_id", "name", "content_type",
	"size_bytes", "s3_key", "version", "parent_id", "created_at", "updated_at",
}

// newTestFilesHandler builds a FilesHandler backed by a sqlmock database.
// The caller is responsible for closing the returned db.
func newTestFilesHandler(t *testing.T) (*FilesHandler, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	st := store.NewWithDB(db)
	h := NewFilesHandler(st, nil)
	return h, mock, func() { db.Close() }
}

// setTenantID injects the tenant_id into the Gin context the same way
// AuthMiddleware + TenantMiddleware would.
func setTenantID(c *gin.Context, tenantID string) {
	c.Set(middleware.ContextKeyTenantID, tenantID)
}

// --- GET /v1/files (List) ---

func TestListFiles_ReturnsListWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(handlerFileColumns).
			AddRow("file-1", "tenant-1", nil, nil, "a.csv", "text/csv",
				int64(100), "key1", 1, nil, now, now).
			AddRow("file-2", "tenant-1", nil, nil, "b.json", "application/json",
				int64(200), "key2", 1, nil, now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	setTenantID(c, "tenant-1")

	h.List(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var files []store.File
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].ID != "file-1" {
		t.Errorf("files[0].ID = %q, want %q", files[0].ID, "file-1")
	}
	if files[1].ID != "file-2" {
		t.Errorf("files[1].ID = %q, want %q", files[1].ID, "file-2")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFiles_ReturnsEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	setTenantID(c, "tenant-1")

	h.List(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Should return [] not null.
	body := w.Body.String()
	if body != "[]" {
		t.Errorf("body = %s, want []", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFiles_WithQueryFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "sess-99", "exec-42", 10, 5).
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files?session_id=sess-99&execution_id=exec-42&limit=10&offset=5", nil)
	setTenantID(c, "tenant-1")

	h.List(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFiles_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WillReturnError(http.ErrServerClosed)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	setTenantID(c, "tenant-1")

	h.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- GET /v1/files/:id (Get) ---

func TestGetFile_ReturnsSingleFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns).
			AddRow("file-uuid-1", "tenant-1", "sess-1", "exec-1", "report.csv", "text/csv",
				int64(1024), "key1", 1, nil, now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files/file-uuid-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "file-uuid-1"}}
	setTenantID(c, "tenant-1")

	h.Get(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var f store.File
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if f.ID != "file-uuid-1" {
		t.Errorf("ID = %q, want %q", f.ID, "file-uuid-1")
	}
	if f.Name != "report.csv" {
		t.Errorf("Name = %q, want %q", f.Name, "report.csv")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetFile_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files/missing-uuid", nil)
	c.Params = gin.Params{{Key: "id", Value: "missing-uuid"}}
	setTenantID(c, "tenant-1")

	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetFile_TenantIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	// The file belongs to tenant-2 but the caller is tenant-1.
	// With tenant_id in the SQL WHERE clause, the query returns no rows.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files/file-uuid-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "file-uuid-1"}}
	setTenantID(c, "tenant-1")

	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetFile_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _, cleanup := newTestFilesHandler(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/files/", nil)
	// Do not set param "id" — simulates empty.
	setTenantID(c, "tenant-1")

	h.Get(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- DELETE /v1/files/:id (Delete) ---

func TestDeleteFile_Returns204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	// First: GetFile to verify ownership.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns).
			AddRow("file-uuid-1", "tenant-1", nil, nil, "doomed.txt", "text/plain",
				int64(50), "key1", 1, nil, now, now))

	// Second: DeleteFile by ID and tenant.
	mock.ExpectExec("DELETE FROM sandbox.files").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Use a gin router so the response status is properly flushed to the
	// recorder. gin's c.Status() only sets the status on the gin writer;
	// it does not call WriteHeader on the underlying httptest.Recorder
	// until the router finalizes the response.
	router := gin.New()
	router.DELETE("/v1/files/:id", func(c *gin.Context) {
		setTenantID(c, "tenant-1")
		h.Delete(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/files/file-uuid-1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/v1/files/missing-uuid", nil)
	c.Params = gin.Params{{Key: "id", Value: "missing-uuid"}}
	setTenantID(c, "tenant-1")

	h.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteFile_TenantIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mock, cleanup := newTestFilesHandler(t)
	defer cleanup()

	// File belongs to tenant-2, but caller is tenant-1.
	// With tenant_id in the SQL WHERE clause, the query returns no rows.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnRows(sqlmock.NewRows(handlerFileColumns))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/v1/files/file-uuid-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "file-uuid-1"}}
	setTenantID(c, "tenant-1")

	h.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (tenant isolation)", w.Code, http.StatusNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- POST /v1/files (Upload) ---

func TestUpload_MissingFileField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _, cleanup := newTestFilesHandler(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No body at all — c.Request.FormFile("file") will return an error.
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/files", nil)
	setTenantID(c, "tenant-1")

	h.Upload(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if resp["error"] != "bad_request" {
		t.Errorf("error = %q, want %q", resp["error"], "bad_request")
	}
}

func TestUpload_MissingFileName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _, cleanup := newTestFilesHandler(t)
	defer cleanup()

	// Build a multipart body with a "file" part whose Content-Disposition
	// carries an empty filename. This passes the FormFile("file") check but
	// causes the name-resolution logic to fall through to the 400 branch,
	// because both the "name" form field and header.Filename are empty.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Craft the part header manually so the filename is explicitly empty.
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename=""`)
	partHeader.Set("Content-Type", "application/octet-stream")
	part, err := mw.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	// Write minimal content so the part is well-formed.
	if _, err = part.Write([]byte("data")); err != nil {
		t.Fatalf("failed to write part content: %v", err)
	}
	mw.Close() //nolint:errcheck

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req
	setTenantID(c, "tenant-1")

	h.Upload(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if resp["error"] != "bad_request" {
		t.Errorf("error = %q, want %q", resp["error"], "bad_request")
	}
}
