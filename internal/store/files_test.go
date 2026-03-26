package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// columns used across all file query expectations.
var fileColumns = []string{
	"id", "tenant_id", "session_id", "execution_id", "name", "content_type",
	"size_bytes", "s3_key", "version", "parent_id", "created_at", "updated_at",
}

// --- CreateFile ---

func TestCreateFile_InsertsAndReturnsFile(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("INSERT INTO sandbox.files").
		WithArgs("tenant-1", "sess-1", "exec-1", "report.csv", "text/csv",
			int64(1024), "tenant-1/exec-1/v1/report.csv", 1, "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("file-uuid-1", now, now))

	f := &File{
		TenantID:    "tenant-1",
		SessionID:   "sess-1",
		ExecutionID: "exec-1",
		Name:        "report.csv",
		ContentType: "text/csv",
		SizeBytes:   1024,
		S3Key:       "tenant-1/exec-1/v1/report.csv",
		Version:     1,
	}

	result, err := s.CreateFile(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "file-uuid-1" {
		t.Errorf("ID = %q, want %q", result.ID, "file-uuid-1")
	}
	if !result.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", result.CreatedAt, now)
	}
	if !result.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", result.UpdatedAt, now)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateFile_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("INSERT INTO sandbox.files").
		WillReturnError(context.DeadlineExceeded)

	f := &File{
		TenantID: "tenant-1",
		Name:     "fail.txt",
	}

	_, err = s.CreateFile(context.Background(), f)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- GetFile ---

func TestGetFile_ReturnsRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	parentID := "parent-uuid"

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-1", "tenant-1", "sess-1", "exec-1", "report.csv", "text/csv",
				int64(1024), "tenant-1/exec-1/v1/report.csv", 1, parentID, now, now))

	f, err := s.GetFile(context.Background(), "file-uuid-1", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ID != "file-uuid-1" {
		t.Errorf("ID = %q, want %q", f.ID, "file-uuid-1")
	}
	if f.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", f.TenantID, "tenant-1")
	}
	if f.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", f.SessionID, "sess-1")
	}
	if f.ExecutionID != "exec-1" {
		t.Errorf("ExecutionID = %q, want %q", f.ExecutionID, "exec-1")
	}
	if f.Name != "report.csv" {
		t.Errorf("Name = %q, want %q", f.Name, "report.csv")
	}
	if f.ContentType != "text/csv" {
		t.Errorf("ContentType = %q, want %q", f.ContentType, "text/csv")
	}
	if f.SizeBytes != 1024 {
		t.Errorf("SizeBytes = %d, want %d", f.SizeBytes, 1024)
	}
	if f.S3Key != "tenant-1/exec-1/v1/report.csv" {
		t.Errorf("S3Key = %q, want %q", f.S3Key, "tenant-1/exec-1/v1/report.csv")
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want %d", f.Version, 1)
	}
	if f.ParentID == nil || *f.ParentID != parentID {
		t.Errorf("ParentID = %v, want %q", f.ParentID, parentID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetFile_NullOptionalFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("file-uuid-2", "tenant-1").
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-uuid-2", "tenant-1", nil, nil, "standalone.txt", "text/plain",
				int64(256), "tenant-1/standalone.txt", 1, nil, now, now))

	f, err := s.GetFile(context.Background(), "file-uuid-2", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", f.SessionID)
	}
	if f.ExecutionID != "" {
		t.Errorf("ExecutionID = %q, want empty", f.ExecutionID)
	}
	if f.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", f.ParentID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetFile_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("missing-uuid", "tenant-1").
		WillReturnRows(sqlmock.NewRows(fileColumns))

	_, err = s.GetFile(context.Background(), "missing-uuid", "tenant-1")
	if err != ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- ListFiles ---

func TestListFiles_FiltersByTenantSessionExecution(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "sess-1", "exec-1", 50, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-1", "tenant-1", "sess-1", "exec-1", "a.csv", "text/csv",
				int64(100), "key1", 1, nil, now, now).
			AddRow("file-2", "tenant-1", "sess-1", "exec-1", "b.json", "application/json",
				int64(200), "key2", 1, nil, now, now))

	files, err := s.ListFiles(context.Background(), FileFilter{
		TenantID:    "tenant-1",
		SessionID:   "sess-1",
		ExecutionID: "exec-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestListFiles_DefaultLimitAndOffset(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	// When Limit <= 0, the code defaults to 50; when Offset < 0, defaults to 0.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "", "", 50, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	files, err := s.ListFiles(context.Background(), FileFilter{
		TenantID: "tenant-1",
		Limit:    -1,
		Offset:   -5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files != nil {
		t.Errorf("files = %v, want nil (no rows)", files)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFiles_CapsLimitAt200(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	// Limit > 200 should be clamped to 200.
	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WithArgs("tenant-1", "", "", 200, 0).
		WillReturnRows(sqlmock.NewRows(fileColumns))

	_, err = s.ListFiles(context.Background(), FileFilter{
		TenantID: "tenant-1",
		Limit:    999,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFiles_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("SELECT id, tenant_id, session_id, execution_id, name, content_type").
		WillReturnError(context.DeadlineExceeded)

	_, err = s.ListFiles(context.Background(), FileFilter{TenantID: "tenant-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- UpdateFile ---

func TestUpdateFile_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("UPDATE sandbox.files").
		WithArgs("file-uuid-1", "renamed.csv", "text/csv", int64(2048),
			"tenant-1/exec-1/v2/renamed.csv", 2, "").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = s.UpdateFile(context.Background(), &File{
		ID:          "file-uuid-1",
		Name:        "renamed.csv",
		ContentType: "text/csv",
		SizeBytes:   2048,
		S3Key:       "tenant-1/exec-1/v2/renamed.csv",
		Version:     2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateFile_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("UPDATE sandbox.files").
		WithArgs("missing-uuid", "x.txt", "text/plain", int64(0), "", 1, "").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = s.UpdateFile(context.Background(), &File{
		ID:          "missing-uuid",
		Name:        "x.txt",
		ContentType: "text/plain",
		Version:     1,
	})
	if err != ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateFile_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("UPDATE sandbox.files").
		WillReturnError(context.DeadlineExceeded)

	err = s.UpdateFile(context.Background(), &File{ID: "file-uuid-1", Name: "x.txt", ContentType: "text/plain", Version: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- DeleteFile ---

func TestDeleteFile_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("DELETE FROM sandbox.files").
		WithArgs("file-uuid-1", "tenant-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = s.DeleteFile(context.Background(), "file-uuid-1", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("DELETE FROM sandbox.files").
		WithArgs("missing-uuid", "tenant-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = s.DeleteFile(context.Background(), "missing-uuid", "tenant-1")
	if err != ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteFile_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("DELETE FROM sandbox.files").
		WillReturnError(context.DeadlineExceeded)

	err = s.DeleteFile(context.Background(), "file-uuid-1", "tenant-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// --- ListFileVersions ---

func TestListFileVersions_ReturnsVersionChain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	rootID := "file-root"

	mock.ExpectQuery("WITH RECURSIVE root AS").
		WithArgs("file-root", "tenant-1").
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow("file-v2", "tenant-1", "sess-1", "exec-1", "report.csv", "text/csv",
				int64(2048), "key-v2", 2, rootID, now, now).
			AddRow("file-root", "tenant-1", "sess-1", "exec-1", "report.csv", "text/csv",
				int64(1024), "key-v1", 1, nil, now, now))

	versions, err := s.ListFileVersions(context.Background(), "file-root", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
	// Should be ordered by version DESC (newest first).
	if versions[0].Version != 2 {
		t.Errorf("versions[0].Version = %d, want 2", versions[0].Version)
	}
	if versions[1].Version != 1 {
		t.Errorf("versions[1].Version = %d, want 1", versions[1].Version)
	}
	if versions[0].ParentID == nil || *versions[0].ParentID != rootID {
		t.Errorf("versions[0].ParentID = %v, want %q", versions[0].ParentID, rootID)
	}
	if versions[1].ParentID != nil {
		t.Errorf("versions[1].ParentID = %v, want nil", versions[1].ParentID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFileVersions_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("WITH RECURSIVE root AS").
		WithArgs("nonexistent", "tenant-1").
		WillReturnRows(sqlmock.NewRows(fileColumns))

	versions, err := s.ListFileVersions(context.Background(), "nonexistent", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if versions != nil {
		t.Errorf("versions = %v, want nil (no rows)", versions)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestListFileVersions_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("WITH RECURSIVE root AS").
		WillReturnError(context.DeadlineExceeded)

	_, err = s.ListFileVersions(context.Background(), "file-root", "tenant-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
