// Package backfill holds idempotent boot-time data migrations that need both DB and S3 access.
package backfill

import (
	"context"
	"log/slog"

	"github.com/devs-group/skillbox/internal/store"
)

// LegacyVersion is the placeholder version assigned to skills imported before the version system.
const LegacyVersion = "0.0.0"

// TargetVersion is the semver legacy skills are repointed to.
const TargetVersion = "1.0.0"

// artifactMover is the subset of registry the backfill needs.
type artifactMover interface {
	CopyVersion(ctx context.Context, tenantID, skillName, from, to string) error
	Delete(ctx context.Context, tenantID, skillName, version string) error
}

// skillStore is the subset of store the backfill needs.
type skillStore interface {
	ListSkillsAtVersion(ctx context.Context, version string) ([]store.SkillRef, error)
	SkillVersionExists(ctx context.Context, tenantID, name, version string) (bool, error)
	RenameSkillVersion(ctx context.Context, tenantID, name, from, to string) error
	BackfillActiveVersions(ctx context.Context) (int64, error)
}

// Result summarizes one backfill pass.
type Result struct {
	VersionsRenamed int
	ActiveBackfilled int64
	Skipped         int
	Failed          int
}

// SkillVersions repoints legacy 0.0.0 skills to 1.0.0 (S3 copy + DB rename + old-key delete),
// then backfills the is_active pointer. Idempotent and failure-tolerant: per-skill errors are
// logged and the pass continues, so a later boot retries what's left.
func SkillVersions(ctx context.Context, st skillStore, mv artifactMover, logger *slog.Logger) Result {
	if logger == nil {
		logger = slog.Default()
	}
	var res Result

	legacy, err := st.ListSkillsAtVersion(ctx, LegacyVersion)
	if err != nil {
		logger.Error("skill backfill: failed to list legacy skills", "err", err)
	}
	for _, ref := range legacy {
		// A 1.0.0 already present means we can't repoint without colliding; leave the legacy row as-is.
		exists, err := st.SkillVersionExists(ctx, ref.TenantID, ref.Name, TargetVersion)
		if err != nil {
			logger.Error("skill backfill: version-exists check failed", "tenant", ref.TenantID, "name", ref.Name, "err", err)
			res.Failed++
			continue
		}
		if exists {
			logger.Warn("skill backfill: target version already exists, skipping", "tenant", ref.TenantID, "name", ref.Name, "target", TargetVersion)
			res.Skipped++
			continue
		}
		// Copy S3 first; if the DB rename then fails the old key is untouched, so a retry is safe.
		if err := mv.CopyVersion(ctx, ref.TenantID, ref.Name, LegacyVersion, TargetVersion); err != nil {
			logger.Error("skill backfill: s3 copy failed", "tenant", ref.TenantID, "name", ref.Name, "err", err)
			res.Failed++
			continue
		}
		if err := st.RenameSkillVersion(ctx, ref.TenantID, ref.Name, LegacyVersion, TargetVersion); err != nil {
			logger.Error("skill backfill: db rename failed, leaving legacy key in place", "tenant", ref.TenantID, "name", ref.Name, "err", err)
			res.Failed++
			continue
		}
		if err := mv.Delete(ctx, ref.TenantID, ref.Name, LegacyVersion); err != nil {
			logger.Warn("skill backfill: legacy s3 delete failed, version migrated but old object remains", "tenant", ref.TenantID, "name", ref.Name, "err", err)
		}
		res.VersionsRenamed++
	}

	n, err := st.BackfillActiveVersions(ctx)
	if err != nil {
		logger.Error("skill backfill: active-version backfill failed", "err", err)
	}
	res.ActiveBackfilled = n

	logger.Info("skill backfill: complete",
		"versions_renamed", res.VersionsRenamed,
		"active_backfilled", res.ActiveBackfilled,
		"skipped", res.Skipped,
		"failed", res.Failed)
	return res
}
