//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
)

// newTestStore opens a Store against the integration Postgres and returns it
// with a fresh per-test tenant id so tests stay isolated on a shared database.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dsn := os.Getenv("SKILLBOX_TEST_DB_DSN")
	if dsn == "" {
		if os.Getenv("REQUIRE_TEST_DB") != "" {
			t.Fatal("SKILLBOX_TEST_DB_DSN is required when REQUIRE_TEST_DB is set")
		}
		t.Skip("SKILLBOX_TEST_DB_DSN not set; skipping integration test")
	}
	s, err := New(dsn)
	if err != nil {
		if os.Getenv("REQUIRE_TEST_DB") != "" {
			t.Fatalf("connect test db: %v", err)
		}
		t.Skipf("connect test db: %v", err)
	}
	tenant := "test-" + uuid.NewString()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1`, tenant)
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sandbox.skills WHERE tenant_id = $1`, tenant)
		_ = s.Close()
	})
	return s, tenant
}

// seedVersion inserts one skill version at the given status.
func seedVersion(t *testing.T, s *Store, tenant, name, version, status string) {
	t.Helper()
	if err := s.UpsertSkill(context.Background(), &SkillRecord{
		TenantID: tenant, Name: name, Version: version,
		Description: "d", Lang: "bash", Status: status,
	}); err != nil {
		t.Fatalf("seed %s@%s: %v", name, version, err)
	}
}

func TestIntegration_NextIntakeVersion_AppendsAfterExisting(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "intake-skill"

	v, appended, err := s.NextIntakeVersion(ctx, tenant, name, "1.0.0")
	if err != nil {
		t.Fatalf("first intake: %v", err)
	}
	if appended || v != "1.0.0" {
		t.Fatalf("first intake = (%q, %v), want (1.0.0, false)", v, appended)
	}

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusAvailable)
	if err := s.SetActiveVersion(ctx, tenant, name, "1.0.0"); err != nil {
		t.Fatalf("activate 1.0.0: %v", err)
	}

	v, appended, err = s.NextIntakeVersion(ctx, tenant, name, "1.0.0")
	if err != nil {
		t.Fatalf("second intake: %v", err)
	}
	if !appended || v != "1.0.1" {
		t.Fatalf("second intake = (%q, %v), want (1.0.1, true)", v, appended)
	}

	// A second intake of identical content mints a new distinct version, not a 409.
	seedVersion(t, s, tenant, name, "1.0.1", SkillStatusReview)
	v, appended, err = s.NextIntakeVersion(ctx, tenant, name, "1.0.0")
	if err != nil {
		t.Fatalf("third intake: %v", err)
	}
	if !appended || v != "1.0.2" {
		t.Fatalf("third intake = (%q, %v), want (1.0.2, true)", v, appended)
	}
}

func TestIntegration_NextIntakeVersion_DeclinedOnlyIsFirstInstall(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "declined-skill"

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusDeclined)

	v, appended, err := s.NextIntakeVersion(ctx, tenant, name, "2.0.0")
	if err != nil {
		t.Fatalf("intake: %v", err)
	}
	if appended || v != "2.0.0" {
		t.Fatalf("intake over declined-only = (%q, %v), want (2.0.0, false)", v, appended)
	}
}

func TestIntegration_Lifecycle_BlockFreeze(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "lifecycle-skill"

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusReview)
	seedVersion(t, s, tenant, name, "1.0.1", SkillStatusReview)
	seedVersion(t, s, tenant, name, "1.0.2", SkillStatusReview)

	if err := s.ReviewSkill(ctx, tenant, name, "1.0.2", "approve", "admin", ""); err != nil {
		t.Fatalf("approve 1.0.2: %v", err)
	}
	if err := s.SetActiveVersion(ctx, tenant, name, "1.0.2"); err != nil {
		t.Fatalf("activate 1.0.2: %v", err)
	}

	// Block the version before the approved one. Freezes 1.0.1 and every later version.
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.1", "decline_forever", "admin", "bad"); err != nil {
		t.Fatalf("block 1.0.1: %v", err)
	}

	// Active (approved) version still resolves; runtime is not gated by the freeze.
	if active, err := s.ResolveActiveVersion(ctx, tenant, name); err != nil || active != "1.0.2" {
		t.Fatalf("ResolveActiveVersion = (%q, %v), want (1.0.2, nil)", active, err)
	}

	// The earliest un-reviewed version below the blocked one stays reviewable.
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.0", "approve", "admin", ""); err != nil {
		t.Fatalf("approve 1.0.0 below block: %v", err)
	}

	// A frozen version rejects review.
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.1", "approve", "admin", ""); !errors.Is(err, ErrBlocked) {
		t.Fatalf("review frozen 1.0.1 = %v, want ErrBlocked", err)
	}

	// Reopen unblocks and clears the block by name.
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.1", "reopen", "admin", ""); err != nil {
		t.Fatalf("reopen 1.0.1: %v", err)
	}
	if blocked, err := s.IsSkillBlocked(ctx, tenant, name); err != nil || blocked {
		t.Fatalf("IsSkillBlocked after reopen = (%v, %v), want (false, nil)", blocked, err)
	}
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.1", "approve", "admin", ""); err != nil {
		t.Fatalf("approve 1.0.1 after reopen: %v", err)
	}
}

func TestIntegration_DeclineRepointsActive(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "repoint-skill"

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusAvailable)
	seedVersion(t, s, tenant, name, "1.0.1", SkillStatusAvailable)
	if err := s.SetActiveVersion(ctx, tenant, name, "1.0.0"); err != nil {
		t.Fatalf("activate 1.0.0: %v", err)
	}

	if err := s.ReviewSkill(ctx, tenant, name, "1.0.0", "decline", "admin", ""); err != nil {
		t.Fatalf("decline active 1.0.0: %v", err)
	}

	if active, err := s.ResolveActiveVersion(ctx, tenant, name); err != nil || active != "1.0.1" {
		t.Fatalf("ResolveActiveVersion after decline = (%q, %v), want (1.0.1, nil)", active, err)
	}
}

func TestIntegration_DeleteSkillAllVersions_DropsHistoryAndBlock(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "delete-all-skill"

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusAvailable)
	seedVersion(t, s, tenant, name, "1.0.1", SkillStatusReview)
	if err := s.ReviewSkill(ctx, tenant, name, "1.0.1", "decline_forever", "admin", "bad"); err != nil {
		t.Fatalf("block 1.0.1: %v", err)
	}

	if err := s.DeleteSkillAllVersions(ctx, tenant, name); err != nil {
		t.Fatalf("DeleteSkillAllVersions: %v", err)
	}

	versions, err := s.ListSkillVersions(ctx, tenant, name)
	if err != nil {
		t.Fatalf("ListSkillVersions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("versions after delete = %d, want 0", len(versions))
	}
	if blocked, err := s.IsSkillBlocked(ctx, tenant, name); err != nil || blocked {
		t.Fatalf("IsSkillBlocked after delete = (%v, %v), want (false, nil)", blocked, err)
	}

	// Re-adding the name after a full delete behaves like a first install.
	v, appended, err := s.NextIntakeVersion(ctx, tenant, name, "1.0.0")
	if err != nil || appended || v != "1.0.0" {
		t.Fatalf("re-add intake = (%q, %v, %v), want (1.0.0, false, nil)", v, appended, err)
	}
}

func TestIntegration_ResolveActiveVersion_IgnoresNonAvailableActive(t *testing.T) {
	s, tenant := newTestStore(t)
	ctx := context.Background()
	const name = "stale-active-skill"

	seedVersion(t, s, tenant, name, "1.0.0", SkillStatusAvailable)
	seedVersion(t, s, tenant, name, "1.0.1", SkillStatusPending)

	// Force a pending version to hold the active pointer (a state the API never reaches via SetActiveVersion).
	if _, err := s.db.ExecContext(ctx,
		`UPDATE sandbox.skills SET is_active = (version = '1.0.1') WHERE tenant_id = $1 AND name = $2`,
		tenant, name); err != nil {
		t.Fatalf("force stale active: %v", err)
	}

	// A non-available active pointer is ignored; resolution falls back to newest available.
	if active, err := s.ResolveActiveVersion(ctx, tenant, name); err != nil || active != "1.0.0" {
		t.Fatalf("ResolveActiveVersion = (%q, %v), want (1.0.0, nil)", active, err)
	}
}
