package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResolveLatestVersion_ReturnsLatest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

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


