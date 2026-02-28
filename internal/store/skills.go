package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SkillRecord represents a row in the sandbox.skills metadata table.
type SkillRecord struct {
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Lang        string    `json:"lang"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// UpsertSkill inserts or updates a skill metadata record. On conflict
// (same tenant, name, version) it updates the description and lang.
func (s *Store) UpsertSkill(ctx context.Context, rec *SkillRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sandbox.skills (tenant_id, name, version, description, lang)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, name, version)
		DO UPDATE SET description = EXCLUDED.description,
		              lang = EXCLUDED.lang,
		              uploaded_at = now()
	`, rec.TenantID, rec.Name, rec.Version, rec.Description, rec.Lang)
	if err != nil {
		return fmt.Errorf("upsert skill: %w", err)
	}
	return nil
}

// ListSkills returns all skill metadata for a tenant, ordered by name
// then version.
func (s *Store) ListSkills(ctx context.Context, tenantID string) ([]SkillRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, name, version, description, lang, uploaded_at
		FROM sandbox.skills
		WHERE tenant_id = $1
		ORDER BY name, version
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []SkillRecord
	for rows.Next() {
		var rec SkillRecord
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.UploadedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, name, version, description, lang, uploaded_at
		FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND version = $3
	`, tenantID, name, version).Scan(
		&rec.TenantID, &rec.Name, &rec.Version,
		&rec.Description, &rec.Lang, &rec.UploadedAt,
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
	err := s.db.QueryRowContext(ctx, `
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

// DeleteSkill removes a skill metadata record.
func (s *Store) DeleteSkill(ctx context.Context, tenantID, name, version string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND version = $3
	`, tenantID, name, version)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	return nil
}
