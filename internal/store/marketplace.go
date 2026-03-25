package store

import (
	"context"
	"fmt"
)

// ListMarketplaceSkills returns public skills, optionally filtered by search
// query (ILIKE on name and description). Returns matching records, total count,
// and any error.
func (s *Store) ListMarketplaceSkills(ctx context.Context, query string, limit, offset int) ([]SkillRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var totalCount int
	var records []SkillRecord

	if query != "" {
		pattern := "%" + query + "%"

		err := s.conn().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sandbox.skills
			WHERE tenant_id = 'public'
			  AND (name ILIKE $1 OR description ILIKE $1)
		`, pattern).Scan(&totalCount)
		if err != nil {
			return nil, 0, fmt.Errorf("count marketplace skills: %w", err)
		}

		rows, err := s.conn().QueryContext(ctx, `
			SELECT tenant_id, name, version, description, lang, uploaded_at
			FROM sandbox.skills
			WHERE tenant_id = 'public'
			  AND (name ILIKE $1 OR description ILIKE $1)
			ORDER BY name, version
			LIMIT $2 OFFSET $3
		`, pattern, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("list marketplace skills: %w", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var rec SkillRecord
			if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version, &rec.Description, &rec.Lang, &rec.UploadedAt); err != nil {
				return nil, 0, fmt.Errorf("scan marketplace skill: %w", err)
			}
			records = append(records, rec)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, fmt.Errorf("iterate marketplace skills: %w", err)
		}
	} else {
		err := s.conn().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sandbox.skills WHERE tenant_id = 'public'
		`).Scan(&totalCount)
		if err != nil {
			return nil, 0, fmt.Errorf("count marketplace skills: %w", err)
		}

		rows, err := s.conn().QueryContext(ctx, `
			SELECT tenant_id, name, version, description, lang, uploaded_at
			FROM sandbox.skills
			WHERE tenant_id = 'public'
			ORDER BY name, version
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("list marketplace skills: %w", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var rec SkillRecord
			if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version, &rec.Description, &rec.Lang, &rec.UploadedAt); err != nil {
				return nil, 0, fmt.Errorf("scan marketplace skill: %w", err)
			}
			records = append(records, rec)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, fmt.Errorf("iterate marketplace skills: %w", err)
		}
	}

	return records, totalCount, nil
}

// GetMarketplaceSkill retrieves a single public skill by name (latest version).
func (s *Store) GetMarketplaceSkill(ctx context.Context, name string) (*SkillRecord, error) {
	rec := &SkillRecord{}
	err := s.conn().QueryRowContext(ctx, `
		SELECT tenant_id, name, version, description, lang, uploaded_at
		FROM sandbox.skills
		WHERE tenant_id = 'public' AND name = $1
		ORDER BY uploaded_at DESC
		LIMIT 1
	`, name).Scan(&rec.TenantID, &rec.Name, &rec.Version, &rec.Description, &rec.Lang, &rec.UploadedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get marketplace skill: %w", err)
	}
	return rec, nil
}
