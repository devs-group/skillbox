package store

import (
	"context"
	"fmt"
)

// SkillRef identifies a skill by tenant and name.
type SkillRef struct {
	TenantID string
	Name     string
}

// ListSkillsAtVersion returns every (tenant, name) that has a row pinned to the given version.
func (s *Store) ListSkillsAtVersion(ctx context.Context, version string) ([]SkillRef, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT tenant_id, name FROM sandbox.skills WHERE version = $1
	`, version)
	if err != nil {
		return nil, fmt.Errorf("list skills at version: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var refs []SkillRef
	for rows.Next() {
		var r SkillRef
		if err := rows.Scan(&r.TenantID, &r.Name); err != nil {
			return nil, fmt.Errorf("scan skill ref: %w", err)
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// SkillVersionExists reports whether a specific (tenant, name, version) row exists.
func (s *Store) SkillVersionExists(ctx context.Context, tenantID, name, version string) (bool, error) {
	var exists bool
	err := s.conn().QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM sandbox.skills WHERE tenant_id = $1 AND name = $2 AND version = $3)
	`, tenantID, name, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("skill version exists: %w", err)
	}
	return exists, nil
}

// RenameSkillVersion repoints a skill's version in place across the skills table and any
// approval rows that pinned the old version. Runs in one transaction.
func (s *Store) RenameSkillVersion(ctx context.Context, tenantID, name, from, to string) error {
	return s.RunInTx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `
			UPDATE sandbox.skills SET version = $4
			WHERE tenant_id = $1 AND name = $2 AND version = $3
		`, tenantID, name, from, to); err != nil {
			return fmt.Errorf("rename skill version: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx, `
			UPDATE sandbox.approval_requests SET skill_version = $4
			WHERE tenant_id = $1 AND skill_name = $2 AND skill_version = $3
		`, tenantID, name, from, to); err != nil {
			return fmt.Errorf("rename approval_requests version: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx, `
			UPDATE sandbox.tenant_approved_skills SET skill_version = $4
			WHERE tenant_id = $1 AND skill_name = $2 AND skill_version = $3
		`, tenantID, name, from, to); err != nil {
			return fmt.Errorf("rename tenant_approved_skills version: %w", err)
		}
		return nil
	})
}

// BackfillActiveVersions sets is_active on one version per (tenant, name) that has none,
// mirroring ResolveActiveVersion: newest available, else newest by uploaded_at. Idempotent.
func (s *Store) BackfillActiveVersions(ctx context.Context) (int64, error) {
	res, err := s.conn().ExecContext(ctx, `
		WITH ranked AS (
			SELECT tenant_id, name, version,
				ROW_NUMBER() OVER (
					PARTITION BY tenant_id, name
					ORDER BY (status = 'available') DESC, uploaded_at DESC
				) AS rn
			FROM sandbox.skills s
			WHERE NOT EXISTS (
				SELECT 1 FROM sandbox.skills a
				WHERE a.tenant_id = s.tenant_id AND a.name = s.name AND a.is_active
			)
		)
		UPDATE sandbox.skills t SET is_active = true
		FROM ranked r
		WHERE t.tenant_id = r.tenant_id AND t.name = r.name AND t.version = r.version AND r.rn = 1
	`)
	if err != nil {
		return 0, fmt.Errorf("backfill active versions: %w", err)
	}
	return res.RowsAffected()
}
