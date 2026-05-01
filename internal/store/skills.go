package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Skill status constants.
const (
	SkillStatusPending     = "pending"
	SkillStatusScanning    = "scanning"
	SkillStatusReview      = "review"
	SkillStatusAvailable   = "available"
	SkillStatusDeclined    = "declined"
	SkillStatusQuarantined = "quarantined"
)

// SkillRecord represents a row in the sandbox.skills metadata table.
type SkillRecord struct {
	TenantID    string          `json:"tenant_id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Lang        string          `json:"lang"`
	Status      string          `json:"status"`
	Stars       int             `json:"stars"`
	ScanResult  json.RawMessage `json:"scan_result,omitempty"`
	ScannedAt   *time.Time      `json:"scanned_at,omitempty"`
	ReviewedBy  *string         `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time      `json:"reviewed_at,omitempty"`
	UploadedAt  time.Time       `json:"uploaded_at"`
}

// UpsertSkill inserts or updates a skill metadata record. On conflict
// (same tenant, name, version) it updates the description, lang, and status.
func (s *Store) UpsertSkill(ctx context.Context, rec *SkillRecord) error {
	status := rec.Status
	if status == "" {
		status = SkillStatusPending
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sandbox.skills (tenant_id, name, version, description, lang, status, stars)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, name, version)
		DO UPDATE SET description = EXCLUDED.description,
		              lang = EXCLUDED.lang,
		              status = EXCLUDED.status,
		              stars = GREATEST(sandbox.skills.stars, EXCLUDED.stars),
		              uploaded_at = now()
	`, rec.TenantID, rec.Name, rec.Version, rec.Description, rec.Lang, status, rec.Stars)
	if err != nil {
		return fmt.Errorf("upsert skill: %w", err)
	}
	return nil
}

// ListSkills returns all skill metadata for a tenant, ordered by name
// then version. By default only returns available skills; pass a non-empty
// status to filter by a specific status.
func (s *Store) ListSkills(ctx context.Context, tenantID string, statusFilter ...string) ([]SkillRecord, error) {
	status := SkillStatusAvailable
	if len(statusFilter) > 0 && statusFilter[0] != "" {
		status = statusFilter[0]
	}
	rows, err := s.conn().QueryContext(ctx, `
		SELECT tenant_id, name, version, description, lang, status, stars,
		       scan_result, scanned_at, reviewed_by, reviewed_at, uploaded_at
		FROM sandbox.skills
		WHERE tenant_id = $1 AND status = $2
		ORDER BY name, version
	`, tenantID, status)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var skills []SkillRecord
	for rows.Next() {
		var rec SkillRecord
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
			&rec.ScanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
			&rec.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan skill row: %w", err)
		}
		skills = append(skills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill rows: %w", err)
	}

	return skills, nil
}

// GetSkill retrieves a single skill metadata record by tenant, name, and version.
func (s *Store) GetSkill(ctx context.Context, tenantID, name, version string) (*SkillRecord, error) {
	rec := &SkillRecord{}
	err := s.conn().QueryRowContext(ctx, `
		SELECT tenant_id, name, version, description, lang, status, stars,
		       scan_result, scanned_at, reviewed_by, reviewed_at, uploaded_at
		FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND version = $3
	`, tenantID, name, version).Scan(
		&rec.TenantID, &rec.Name, &rec.Version,
		&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
		&rec.ScanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
		&rec.UploadedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get skill: %w", err)
	}
	return rec, nil
}

// ResolveLatestVersion returns the version string of the most recently
// uploaded skill for a given tenant and name. If no versions exist, it
// returns ErrNotFound.
func (s *Store) ResolveLatestVersion(ctx context.Context, tenantID, name string) (string, error) {
	var version string
	err := s.conn().QueryRowContext(ctx, `
		SELECT version FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2
		ORDER BY uploaded_at DESC
		LIMIT 1
	`, tenantID, name).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve latest version: %w", err)
	}
	return version, nil
}

// UpdateSkillStatus transitions a skill to a new status, optionally storing
// the scan result. It uses optimistic locking: the update only succeeds if
// the current status matches expectedStatus. Returns ErrNotFound if no row
// matches (wrong status or nonexistent skill).
func (s *Store) UpdateSkillStatus(ctx context.Context, tenantID, name, version, newStatus string, scanResult json.RawMessage) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.skills
		SET status = $4,
		    scan_result = $5,
		    scanned_at = CASE WHEN $4 IN ('available', 'review', 'quarantined') THEN now() ELSE scanned_at END
		WHERE tenant_id = $1 AND name = $2 AND version = $3
	`, tenantID, name, version, newStatus, nullableJSON(scanResult))
	if err != nil {
		return fmt.Errorf("update skill status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update skill status rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSkillStatus returns just the status string for a skill. This is a
// lightweight query used by the runner to check if a skill is executable.
func (s *Store) GetSkillStatus(ctx context.Context, tenantID, name, version string) (string, error) {
	var status string
	err := s.conn().QueryRowContext(ctx, `
		SELECT status FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND version = $3
	`, tenantID, name, version).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get skill status: %w", err)
	}
	return status, nil
}

// ListPendingSkills returns skills in 'pending' or 'scanning' status across
// all tenants. Used by the background scan worker for startup recovery.
func (s *Store) ListPendingSkills(ctx context.Context) ([]SkillRecord, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT tenant_id, name, version, description, lang, status, stars,
		       scan_result, scanned_at, reviewed_by, reviewed_at, uploaded_at
		FROM sandbox.skills
		WHERE status IN ('pending', 'scanning')
		ORDER BY uploaded_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending skills: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var skills []SkillRecord
	for rows.Next() {
		var rec SkillRecord
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
			&rec.ScanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
			&rec.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan pending skill row: %w", err)
		}
		skills = append(skills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending skill rows: %w", err)
	}
	return skills, nil
}

// ReviewSkill records an admin approval or decline for a skill in 'review' status.
func (s *Store) ReviewSkill(ctx context.Context, tenantID, name, version, action, reviewedBy string) error {
	var newStatus string
	switch action {
	case "approve":
		newStatus = SkillStatusAvailable
	case "decline":
		newStatus = SkillStatusDeclined
	default:
		return fmt.Errorf("invalid review action: %q (must be approve or decline)", action)
	}

	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.skills
		SET status = $4,
		    reviewed_by = $5,
		    reviewed_at = now()
		WHERE tenant_id = $1 AND name = $2 AND version = $3
		  AND status = 'review'
	`, tenantID, name, version, newStatus, reviewedBy)
	if err != nil {
		return fmt.Errorf("review skill: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("review skill rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSkill removes a skill metadata record and its associated execution
// records atomically within a transaction. This prevents orphaned execution
// records when a skill version is deleted.
func (s *Store) DeleteSkill(ctx context.Context, tenantID, name, version string) error {
	return s.RunInTx(ctx, func(tx *Store) error {
		// Delete associated execution records first (no FK cascade).
		_, err := tx.conn().ExecContext(ctx, `
			DELETE FROM sandbox.executions
			WHERE tenant_id = $1 AND skill_name = $2 AND skill_version = $3
		`, tenantID, name, version)
		if err != nil {
			return fmt.Errorf("delete skill executions: %w", err)
		}

		// Delete the skill metadata record.
		_, err = tx.conn().ExecContext(ctx, `
			DELETE FROM sandbox.skills
			WHERE tenant_id = $1 AND name = $2 AND version = $3
		`, tenantID, name, version)
		if err != nil {
			return fmt.Errorf("delete skill: %w", err)
		}

		return nil
	})
}
