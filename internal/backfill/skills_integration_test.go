//go:build integration

package backfill

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/devs-group/skillbox/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("SKILLBOX_TEST_DB_DSN")
	if dsn == "" {
		if os.Getenv("REQUIRE_TEST_DB") != "" {
			t.Fatal("SKILLBOX_TEST_DB_DSN is required when REQUIRE_TEST_DB is set")
		}
		t.Skip("SKILLBOX_TEST_DB_DSN not set; skipping integration test")
	}
	s, err := store.New(dsn)
	if err != nil {
		if os.Getenv("REQUIRE_TEST_DB") != "" {
			t.Fatalf("connect test db: %v", err)
		}
		t.Skipf("connect test db: %v", err)
	}
	return s
}

type fakeMover struct {
	copied  map[string]bool
	deleted map[string]bool
}

func newFakeMover() *fakeMover {
	return &fakeMover{copied: map[string]bool{}, deleted: map[string]bool{}}
}

func (f *fakeMover) CopyVersion(_ context.Context, tenantID, name, from, to string) error {
	f.copied[tenantID+"/"+name+"/"+from+"->"+to] = true
	return nil
}

func (f *fakeMover) Delete(_ context.Context, tenantID, name, version string) error {
	f.deleted[tenantID+"/"+name+"/"+version] = true
	return nil
}

func TestSkillVersions_BackfillRenamesAndActivates(t *testing.T) {
	st := newStore(t)
	defer st.Close() //nolint:errcheck
	ctx := context.Background()
	db := st.DB()

	tenant := "t-" + uuid.NewString()

	// Legacy skill awaiting migration, plus an approval pinned to 0.0.0.
	if _, err := db.ExecContext(ctx, `INSERT INTO sandbox.skills (tenant_id, name, version, status, is_active) VALUES ($1,'docx','0.0.0','available',false)`, tenant); err != nil {
		t.Fatal(err)
	}
	var userID string
	if err := db.QueryRowContext(ctx, `INSERT INTO sandbox.users (kratos_identity_id, tenant_id, email) VALUES ($1,$2,$3) RETURNING id`, uuid.NewString(), tenant, tenant+"@test.local").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sandbox.approval_requests (tenant_id, user_id, skill_name, skill_version, status) VALUES ($1,$2,'docx','0.0.0','pending')`, tenant, userID); err != nil {
		t.Fatal(err)
	}

	// Collision case: both 0.0.0 and 1.0.0 already exist; legacy must be left untouched.
	collide := "c-" + uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO sandbox.skills (tenant_id, name, version, status, is_active) VALUES ($1,'pdf','0.0.0','available',false),($1,'pdf','1.0.0','available',false)`, collide); err != nil {
		t.Fatal(err)
	}

	mv := newFakeMover()
	res := SkillVersions(ctx, st, mv, nil)

	if res.VersionsRenamed != 1 {
		t.Errorf("VersionsRenamed = %d, want 1", res.VersionsRenamed)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", res.Skipped)
	}
	if res.Failed != 0 {
		t.Errorf("Failed = %d, want 0", res.Failed)
	}

	// Legacy skill repointed to 1.0.0 and made active.
	var version string
	var active bool
	if err := db.QueryRowContext(ctx, `SELECT version, is_active FROM sandbox.skills WHERE tenant_id=$1 AND name='docx'`, tenant).Scan(&version, &active); err != nil {
		t.Fatal(err)
	}
	if version != "1.0.0" || !active {
		t.Errorf("docx version=%q active=%v, want 1.0.0/true", version, active)
	}

	// Approval row repointed.
	var apprVersion string
	if err := db.QueryRowContext(ctx, `SELECT skill_version FROM sandbox.approval_requests WHERE tenant_id=$1 AND skill_name='docx'`, tenant).Scan(&apprVersion); err != nil {
		t.Fatal(err)
	}
	if apprVersion != "1.0.0" {
		t.Errorf("approval version=%q, want 1.0.0", apprVersion)
	}

	// S3 copy + delete happened for the renamed skill only.
	if !mv.copied[tenant+"/docx/0.0.0->1.0.0"] {
		t.Error("expected S3 copy for docx")
	}
	if !mv.deleted[tenant+"/docx/0.0.0"] {
		t.Error("expected S3 delete of legacy docx")
	}
	if mv.copied[collide+"/pdf/0.0.0->1.0.0"] {
		t.Error("collision skill must not be copied")
	}

	// Collision legacy row still 0.0.0; the pre-existing 1.0.0 became active.
	var collideLegacy bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sandbox.skills WHERE tenant_id=$1 AND name='pdf' AND version='0.0.0')`, collide).Scan(&collideLegacy); err != nil {
		t.Fatal(err)
	}
	if !collideLegacy {
		t.Error("collision legacy 0.0.0 row should remain")
	}
}
