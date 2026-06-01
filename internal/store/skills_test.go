package store

import (
	"context"
	"database/sql"
	"errors"
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

func TestResolveActiveVersion_ReturnsActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("1.0.2"))

	version, err := s.ResolveActiveVersion(context.Background(), "tenant-1", "my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "1.0.2" {
		t.Errorf("version = %q, want %q", version, "1.0.2")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestResolveActiveVersion_FallsBackToLatest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	// no available active pointer, no available version: falls back to newest-any.
	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("2.0.0"))

	version, err := s.ResolveActiveVersion(context.Background(), "tenant-1", "my-skill")
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

func TestResolveActiveVersion_FallbackPrefersAvailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	// no available active pointer, but an available version exists: a newer pending edit must not shadow it.
	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectQuery("SELECT version FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("1.0.0"))

	version, err := s.ResolveActiveVersion(context.Background(), "tenant-1", "my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want %q", version, "1.0.0")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSetActiveVersion_Available(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.2").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(SkillStatusAvailable))
	mock.ExpectExec("UPDATE sandbox.skills SET is_active = false").
		WithArgs("tenant-1", "my-skill").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE sandbox.skills SET is_active = true").
		WithArgs("tenant-1", "my-skill", "1.0.2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := s.SetActiveVersion(context.Background(), "tenant-1", "my-skill", "1.0.2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSetActiveVersion_RejectsNonAvailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.3").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(SkillStatusQuarantined))
	mock.ExpectRollback()

	err = s.SetActiveVersion(context.Background(), "tenant-1", "my-skill", "1.0.3")
	if !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("error = %v, want ErrInvalidStatus", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSetActiveVersion_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM sandbox.skills").
		WithArgs("tenant-1", "my-skill", "9.9.9").
		WillReturnRows(sqlmock.NewRows([]string{"status"}))
	mock.ExpectRollback()

	err = s.SetActiveVersion(context.Background(), "tenant-1", "my-skill", "9.9.9")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestReviewSkill_DeclineActiveRepointsToNewestAvailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("SELECT version FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectExec("UPDATE sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.3", SkillStatusDeclined, "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("UPDATE sandbox.skills SET is_active = false").
		WithArgs("tenant-1", "my-skill", "1.0.3").
		WillReturnRows(sqlmock.NewRows([]string{"bool"}).AddRow(true))
	mock.ExpectExec("UPDATE sandbox.skills SET is_active = true").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.ReviewSkill(context.Background(), "tenant-1", "my-skill", "1.0.3", "decline", "admin", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestReviewSkill_DeclineNonActiveLeavesPointer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectExec("UPDATE sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.3", SkillStatusDeclined, "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// version was not active -> no row returned -> no repoint update.
	mock.ExpectQuery("UPDATE sandbox.skills SET is_active = false").
		WithArgs("tenant-1", "my-skill", "1.0.3").
		WillReturnError(sql.ErrNoRows)

	if err := s.ReviewSkill(context.Background(), "tenant-1", "my-skill", "1.0.3", "decline", "admin", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}



func TestReviewSkill_DeclineForeverDeclinesVersionAndBlocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck

	s := &Store{db: db}

	mock.ExpectQuery("SELECT version FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectExec("UPDATE sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.1", SkillStatusDeclined, "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// active version cleared + repointed to newest available, then the tenant block is inserted.
	mock.ExpectQuery("UPDATE sandbox.skills SET is_active = false").
		WithArgs("tenant-1", "my-skill", "1.0.1").
		WillReturnRows(sqlmock.NewRows([]string{"bool"}).AddRow(true))
	mock.ExpectExec("UPDATE sandbox.skills SET is_active = true").
		WithArgs("tenant-1", "my-skill", SkillStatusAvailable).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill", "1.0.1", "admin", "").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.ReviewSkill(context.Background(), "tenant-1", "my-skill", "1.0.1", "decline_forever", "admin", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestVersionFrozen(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck
	s := &Store{db: db}

	// 1.0.3 is at or after the blocked 1.0.2 -> frozen.
	mock.ExpectQuery("SELECT version FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("1.0.2"))
	if frozen, err := s.versionFrozen(context.Background(), "tenant-1", "my-skill", "1.0.3"); err != nil || !frozen {
		t.Errorf("versionFrozen(1.0.3) = %v,%v; want true,nil", frozen, err)
	}

	// 1.0.1 is before the blocked 1.0.2 -> not frozen.
	mock.ExpectQuery("SELECT version FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("1.0.2"))
	if frozen, err := s.versionFrozen(context.Background(), "tenant-1", "my-skill", "1.0.1"); err != nil || frozen {
		t.Errorf("versionFrozen(1.0.1) = %v,%v; want false,nil", frozen, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestReviewSkill_FrozenVersionRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck
	s := &Store{db: db}

	// Approving the blocked version is refused before any status update.
	mock.ExpectQuery("SELECT version FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("1.0.2"))

	err = s.ReviewSkill(context.Background(), "tenant-1", "my-skill", "1.0.2", "approve", "admin", "")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("error = %v, want ErrBlocked", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestReviewSkill_ReopenBypassesFreezeAndUnblocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close() //nolint:errcheck
	s := &Store{db: db}

	// reopen never checks the freeze (no tenant_blocked_skills SELECT); it updates status and clears the block by name.
	mock.ExpectExec("UPDATE sandbox.skills").
		WithArgs("tenant-1", "my-skill", "1.0.2", SkillStatusReview, "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM sandbox.tenant_blocked_skills").
		WithArgs("tenant-1", "my-skill").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.ReviewSkill(context.Background(), "tenant-1", "my-skill", "1.0.2", "reopen", "admin", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
