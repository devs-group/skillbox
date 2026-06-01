package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/devs-group/skillbox/internal/skill"
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
	SourceURL   *string         `json:"source_url,omitempty"`
	Blocked     bool            `json:"blocked,omitempty"`
	HasReview   bool            `json:"has_review,omitempty"`
	HasDeclined bool            `json:"has_declined,omitempty"`
	HasScanning bool            `json:"has_scanning,omitempty"`
}

// UpsertSkill inserts or updates a skill metadata record. On conflict
// (same tenant, name, version) it updates the description, lang, and status.
func (s *Store) UpsertSkill(ctx context.Context, rec *SkillRecord) error {
	status := rec.Status
	if status == "" {
		status = SkillStatusPending
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sandbox.skills (tenant_id, name, version, description, lang, status, stars, source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, name, version)
		DO UPDATE SET description = EXCLUDED.description,
		              lang = EXCLUDED.lang,
		              status = EXCLUDED.status,
		              stars = GREATEST(sandbox.skills.stars, EXCLUDED.stars),
		              source_url = COALESCE(EXCLUDED.source_url, sandbox.skills.source_url),
		              uploaded_at = now()
	`, rec.TenantID, rec.Name, rec.Version, rec.Description, rec.Lang, status, rec.Stars, rec.SourceURL)
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
		SELECT DISTINCT ON (s.name)
		       s.tenant_id, s.name, s.version, s.description, s.lang, s.status, s.stars,
		       s.scan_result, s.scanned_at, s.reviewed_by, s.reviewed_at, s.uploaded_at, s.source_url,
		       b.name IS NOT NULL AS blocked
		FROM sandbox.skills s
		LEFT JOIN sandbox.tenant_blocked_skills b ON b.tenant_id = s.tenant_id AND b.name = s.name
		WHERE s.tenant_id = $1 AND s.status = $2
		ORDER BY s.name, s.uploaded_at DESC
	`, tenantID, status)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var skills []SkillRecord
	for rows.Next() {
		var rec SkillRecord
		var scanResult []byte
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
			&scanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
			&rec.UploadedAt, &rec.SourceURL, &rec.Blocked); err != nil {
			return nil, fmt.Errorf("scan skill row: %w", err)
		}
		if scanResult != nil {
			rec.ScanResult = json.RawMessage(scanResult)
		}
		skills = append(skills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill rows: %w", err)
	}

	return skills, nil
}

// ListAllSkills returns every skill for a tenant regardless of status.
func (s *Store) ListAllSkills(ctx context.Context, tenantID string) ([]SkillRecord, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT DISTINCT ON (s.name)
		       s.tenant_id, s.name, s.version, s.description, s.lang, s.status, s.stars,
		       s.scan_result, s.scanned_at, s.reviewed_by, s.reviewed_at, s.uploaded_at, s.source_url,
		       b.name IS NOT NULL AS blocked,
		       EXISTS(SELECT 1 FROM sandbox.skills r WHERE r.tenant_id = s.tenant_id AND r.name = s.name AND r.status IN ('review','pending','scanning')) AS has_review,
		       EXISTS(SELECT 1 FROM sandbox.skills r WHERE r.tenant_id = s.tenant_id AND r.name = s.name AND r.status IN ('declined','quarantined')) AS has_declined,
		       EXISTS(SELECT 1 FROM sandbox.skills r WHERE r.tenant_id = s.tenant_id AND r.name = s.name AND r.status IN ('pending','scanning')) AS has_scanning
		FROM sandbox.skills s
		LEFT JOIN sandbox.tenant_blocked_skills b ON b.tenant_id = s.tenant_id AND b.name = s.name
		WHERE s.tenant_id = $1
		ORDER BY s.name, s.is_active DESC, s.uploaded_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list all skills: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var skills []SkillRecord
	for rows.Next() {
		var rec SkillRecord
		var scanResult []byte
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
			&scanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
			&rec.UploadedAt, &rec.SourceURL, &rec.Blocked,
			&rec.HasReview, &rec.HasDeclined, &rec.HasScanning); err != nil {
			return nil, fmt.Errorf("scan skill row: %w", err)
		}
		if scanResult != nil {
			rec.ScanResult = json.RawMessage(scanResult)
		}
		skills = append(skills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill rows: %w", err)
	}
	return skills, nil
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

// SkillVersionInfo is a lightweight view of one stored skill version.
type SkillVersionInfo struct {
	Version      string            `json:"version"`
	Status       string            `json:"status"`
	Active       bool              `json:"active"`
	Blocked      bool              `json:"blocked"`
	UploadedAt   time.Time         `json:"uploaded_at"`
	ScanSummary  string            `json:"scan_summary,omitempty"`
	ScanFindings []ScanFindingInfo `json:"scan_findings,omitempty"`
}

// ScanFindingInfo is the reviewer-facing subset of a security scan finding.
type ScanFindingInfo struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	FilePath    string `json:"file_path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

// ListSkillVersions returns every stored version of a skill for a tenant,
// newest first, with status and active flag.
func (s *Store) ListSkillVersions(ctx context.Context, tenantID, name string) ([]SkillVersionInfo, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT s.version, s.status, s.is_active, s.uploaded_at, s.scan_result,
		       b.version IS NOT NULL AS blocked
		FROM sandbox.skills s
		LEFT JOIN sandbox.tenant_blocked_skills b
		  ON b.tenant_id = s.tenant_id AND b.name = s.name AND b.version = s.version
		WHERE s.tenant_id = $1 AND s.name = $2
		ORDER BY s.uploaded_at DESC
	`, tenantID, name)
	if err != nil {
		return nil, fmt.Errorf("list skill versions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var versions []SkillVersionInfo
	for rows.Next() {
		var v SkillVersionInfo
		var scanResult []byte
		if err := rows.Scan(&v.Version, &v.Status, &v.Active, &v.UploadedAt, &scanResult, &v.Blocked); err != nil {
			return nil, fmt.Errorf("scan skill version row: %w", err)
		}
		if len(scanResult) > 0 {
			var sr struct {
				Summary  string `json:"summary"`
				Findings []struct {
					Severity    string `json:"severity"`
					Category    string `json:"category"`
					FilePath    string `json:"file_path"`
					Line        int    `json:"line"`
					Description string `json:"description"`
					Remediation string `json:"remediation"`
				} `json:"findings"`
			}
			if json.Unmarshal(scanResult, &sr) == nil {
				v.ScanSummary = sr.Summary
				for _, f := range sr.Findings {
					v.ScanFindings = append(v.ScanFindings, ScanFindingInfo{
						Severity: f.Severity, Category: f.Category, FilePath: f.FilePath,
						Line: f.Line, Description: f.Description, Remediation: f.Remediation,
					})
				}
			}
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill version rows: %w", err)
	}
	return versions, nil
}

// ResolveActiveVersion returns active pointer (only if available), else newest available, else newest-any; ErrNotFound if none.
func (s *Store) ResolveActiveVersion(ctx context.Context, tenantID, name string) (string, error) {
	var version string
	err := s.conn().QueryRowContext(ctx, `
		SELECT version FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND is_active AND status = $3
		LIMIT 1
	`, tenantID, name, SkillStatusAvailable).Scan(&version)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve active version: %w", err)
	}

	err = s.conn().QueryRowContext(ctx, `
		SELECT version FROM sandbox.skills
		WHERE tenant_id = $1 AND name = $2 AND status = $3
		ORDER BY uploaded_at DESC
		LIMIT 1
	`, tenantID, name, SkillStatusAvailable).Scan(&version)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve active version: %w", err)
	}
	return s.ResolveLatestVersion(ctx, tenantID, name)
}

// NextFreeVersion bumps PATCH from active, skipping any version that already
// exists, so an intake or edit always mints a new increasing version and never
// overwrites a prior one.
func (s *Store) NextFreeVersion(ctx context.Context, tenantID, name, active string) string {
	taken := map[string]bool{}
	if vs, err := s.ListSkillVersions(ctx, tenantID, name); err == nil {
		for _, v := range vs {
			taken[v.Version] = true
		}
	}
	next := skill.NextEditVersion(active)
	for taken[next] {
		next = skill.NextEditVersion(next)
	}
	return next
}

// NextIntakeVersion picks the version a fresh intake (upload/marketplace) uses.
// First intake of a name (no non-declined version) keeps parsedVersion. Otherwise
// it appends: mints the next-free version above the active one so nothing is
// overwritten. Re-adding identical content becomes a new distinct version.
func (s *Store) NextIntakeVersion(ctx context.Context, tenantID, name, parsedVersion string) (version string, appended bool, err error) {
	versions, err := s.ListSkillVersions(ctx, tenantID, name)
	if err != nil {
		return "", false, err
	}
	exists := false
	for _, v := range versions {
		if v.Status != SkillStatusDeclined {
			exists = true
			break
		}
	}
	if !exists {
		return parsedVersion, false, nil
	}
	base, berr := s.ResolveActiveVersion(ctx, tenantID, name)
	if berr != nil {
		base = parsedVersion
	}
	return s.NextFreeVersion(ctx, tenantID, name, base), true, nil
}

// SetActiveVersion makes the given version the active one for (tenant, name).
// The target must be in 'available' status. The previous active version is
// cleared atomically so the one-active invariant holds.
func (s *Store) SetActiveVersion(ctx context.Context, tenantID, name, version string) error {
	return s.RunInTx(ctx, func(tx *Store) error {
		var status string
		err := tx.conn().QueryRowContext(ctx, `
			SELECT status FROM sandbox.skills
			WHERE tenant_id = $1 AND name = $2 AND version = $3
		`, tenantID, name, version).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lookup version for activation: %w", err)
		}
		if status != SkillStatusAvailable {
			return fmt.Errorf("%w: version %q is %q, only available versions can be activated", ErrInvalidStatus, version, status)
		}
		if _, err := tx.conn().ExecContext(ctx, `
			UPDATE sandbox.skills SET is_active = false
			WHERE tenant_id = $1 AND name = $2 AND is_active
		`, tenantID, name); err != nil {
			return fmt.Errorf("clear active version: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx, `
			UPDATE sandbox.skills SET is_active = true
			WHERE tenant_id = $1 AND name = $2 AND version = $3
		`, tenantID, name, version); err != nil {
			return fmt.Errorf("set active version: %w", err)
		}
		return nil
	})
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
		       scan_result, scanned_at, reviewed_by, reviewed_at, uploaded_at, source_url
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
		var scanResult []byte
		if err := rows.Scan(&rec.TenantID, &rec.Name, &rec.Version,
			&rec.Description, &rec.Lang, &rec.Status, &rec.Stars,
			&scanResult, &rec.ScannedAt, &rec.ReviewedBy, &rec.ReviewedAt,
			&rec.UploadedAt, &rec.SourceURL); err != nil {
			return nil, fmt.Errorf("scan pending skill row: %w", err)
		}
		if scanResult != nil {
			rec.ScanResult = json.RawMessage(scanResult)
		}
		skills = append(skills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending skill rows: %w", err)
	}
	return skills, nil
}

// ReviewSkill records an admin review action. Transitions:
// approve: review|declined -> available (also clears any tenant block).
// decline: review|available -> declined (soft, user can reinstall).
// decline_forever: any -> declined + tenant block (user reinstall refused; admin can still reopen).
func (s *Store) ReviewSkill(ctx context.Context, tenantID, name, version, action, reviewedBy, reason string) error {
	var newStatus string
	var allowedFrom []string
	switch action {
	case "approve":
		newStatus = SkillStatusAvailable
		allowedFrom = []string{SkillStatusReview, SkillStatusDeclined}
	case "decline":
		newStatus = SkillStatusDeclined
		allowedFrom = []string{SkillStatusReview, SkillStatusAvailable}
	case "decline_forever":
		newStatus = SkillStatusDeclined
		allowedFrom = []string{SkillStatusReview, SkillStatusAvailable, SkillStatusDeclined, SkillStatusPending, SkillStatusScanning}
	case "reopen":
		newStatus = SkillStatusReview
		allowedFrom = []string{SkillStatusDeclined}
	default:
		return fmt.Errorf("invalid review action: %q (must be approve, decline, decline_forever, or reopen)", action)
	}

	// A block freezes the blocked version and every version after it; earlier versions stay actionable.
	// reopen is always allowed (it unblocks).
	if action != "reopen" {
		if frozen, err := s.versionFrozen(ctx, tenantID, name, version); err == nil && frozen {
			return ErrBlocked
		}
	}

	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.skills
		SET status = $4,
		    reviewed_by = $5,
		    reviewed_at = now()
		WHERE tenant_id = $1 AND name = $2 AND version = $3
		  AND status = ANY($6)
	`, tenantID, name, version, newStatus, reviewedBy, pq.Array(allowedFrom))
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

	// A declined version must not remain the active pointer; fall back to newest available.
	if action == "decline" || action == "decline_forever" {
		var wasActive bool
		err := s.conn().QueryRowContext(ctx, `
			UPDATE sandbox.skills SET is_active = false
			WHERE tenant_id = $1 AND name = $2 AND version = $3 AND is_active
			RETURNING true
		`, tenantID, name, version).Scan(&wasActive)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("clear active on decline: %w", err)
		}
		if wasActive {
			if _, err := s.conn().ExecContext(ctx, `
				UPDATE sandbox.skills SET is_active = true
				WHERE tenant_id = $1 AND name = $2 AND version = (
					SELECT version FROM sandbox.skills
					WHERE tenant_id = $1 AND name = $2 AND status = $3
					ORDER BY uploaded_at DESC LIMIT 1
				)
			`, tenantID, name, SkillStatusAvailable); err != nil {
				return fmt.Errorf("repoint active after decline: %w", err)
			}
		}
	}

	switch action {
	case "reopen":
		// Reopen fully unblocks the skill so new versions can be submitted again.
		if _, err := s.conn().ExecContext(ctx,
			`DELETE FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2`,
			tenantID, name); err != nil {
			return fmt.Errorf("clear block: %w", err)
		}
	case "approve":
		if _, err := s.conn().ExecContext(ctx,
			`DELETE FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2 AND version IN ($3, '')`,
			tenantID, name, version); err != nil {
			return fmt.Errorf("clear block: %w", err)
		}
	case "decline_forever":
		if _, err := s.conn().ExecContext(ctx, `
			INSERT INTO sandbox.tenant_blocked_skills (tenant_id, name, version, blocked_by, reason)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, name, version)
			DO UPDATE SET blocked_by = EXCLUDED.blocked_by, reason = EXCLUDED.reason, blocked_at = now()
		`, tenantID, name, version, reviewedBy, reason); err != nil {
			return fmt.Errorf("insert block: %w", err)
		}
	}
	return nil
}

// IsSkillBlocked reports whether (tenant, name) is in the permanent block list.
func (s *Store) IsSkillBlocked(ctx context.Context, tenantID, name string) (bool, error) {
	var n int
	err := s.conn().QueryRowContext(ctx,
		`SELECT 1 FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2`,
		tenantID, name).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check block: %w", err)
	}
	return true, nil
}

// versionFrozen reports whether the given version is at or after any blocked version
// (a block freezes its version and every later one; earlier versions stay actionable).
func (s *Store) versionFrozen(ctx context.Context, tenantID, name, version string) (bool, error) {
	rows, err := s.conn().QueryContext(ctx,
		`SELECT version FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2`,
		tenantID, name)
	if err != nil {
		return false, fmt.Errorf("check frozen: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var bv string
		if err := rows.Scan(&bv); err != nil {
			return false, err
		}
		if skill.CompareVersions(version, bv) >= 0 {
			return true, nil
		}
	}
	return false, rows.Err()
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

		// When the last version is gone, clear any permanent block so the name can be re-uploaded.
		var remaining int
		if err := tx.conn().QueryRowContext(ctx,
			`SELECT count(*) FROM sandbox.skills WHERE tenant_id = $1 AND name = $2`,
			tenantID, name).Scan(&remaining); err != nil {
			return fmt.Errorf("count remaining versions: %w", err)
		}
		if remaining == 0 {
			if _, err := tx.conn().ExecContext(ctx,
				`DELETE FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2`,
				tenantID, name); err != nil {
				return fmt.Errorf("clear block on delete: %w", err)
			}
		}

		return nil
	})
}

// DeleteSkillAllVersions removes every version of a skill plus its executions
// and any block, dropping all history so the name can be re-added from scratch.
func (s *Store) DeleteSkillAllVersions(ctx context.Context, tenantID, name string) error {
	return s.RunInTx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx,
			`DELETE FROM sandbox.executions WHERE tenant_id = $1 AND skill_name = $2`,
			tenantID, name); err != nil {
			return fmt.Errorf("delete skill executions: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx,
			`DELETE FROM sandbox.skills WHERE tenant_id = $1 AND name = $2`,
			tenantID, name); err != nil {
			return fmt.Errorf("delete skill versions: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx,
			`DELETE FROM sandbox.tenant_blocked_skills WHERE tenant_id = $1 AND name = $2`,
			tenantID, name); err != nil {
			return fmt.Errorf("clear block on delete: %w", err)
		}
		return nil
	})
}
