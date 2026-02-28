package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResolveLatestVersion_ReturnsLatest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	s := &Store{db: db}

	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("2.0.0"))

	version, err := s.ResolveLatestVersion(context.Background(), "tenant-1", "my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "2.0.0" {
		t.Errorf("version = %q, want %q", version, "2.0.0")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestResolveLatestVersion_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	s := &Store{db: db}

	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"version"}))

	_, err = s.ResolveLatestVersion(context.Background(), "tenant-1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetSkill_ReturnsRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	s := &Store{db: db}

	mock.ExpectQuery("SELECT tenant_id, name, version, description, lang, uploaded_at FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.0").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "name", "version", "description", "lang", "uploaded_at"}).
			AddRow("tenant-1", "my-skill", "1.0.0", "A test skill", "python", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	rec, err := s.GetSkill(context.Background(), "tenant-1", "my-skill", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Name != "my-skill" || rec.Version != "1.0.0" {
		t.Errorf("got %s@%s, want my-skill@1.0.0", rec.Name, rec.Version)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetSkill_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	s := &Store{db: db}

	mock.ExpectQuery("SELECT tenant_id, name, version, description, lang, uploaded_at FROM sandbox.skills").
		WithArgs("tenant-1", "missing", "1.0.0").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "name", "version", "description", "lang", "uploaded_at"}))

	_, err = s.GetSkill(context.Background(), "tenant-1", "missing", "1.0.0")
	if err != ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
