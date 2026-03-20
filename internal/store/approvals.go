package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ApprovalRequest represents a row in the sandbox.approval_requests table.
type ApprovalRequest struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	UserID          string     `json:"user_id"`
	SkillName       string     `json:"skill_name"`
	SkillVersion    string     `json:"skill_version"`
	Status          string     `json:"status"`
	AlsoRequestedBy []string   `json:"also_requested_by"`
	ReviewedBy      *string    `json:"reviewed_by,omitempty"`
	ReviewComment   *string    `json:"review_comment,omitempty"`
	ScanResult      *string    `json:"scan_result,omitempty"`
	Source          string     `json:"source"`
	SourceURL       *string    `json:"source_url,omitempty"`
	ApprovalScope   string     `json:"approval_scope"`
	CreatedAt       time.Time  `json:"created_at"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
}

// CreateApprovalRequest creates an approval request. If a request for the
// same skill/version already exists for this tenant, the requesting user
// is added to the also_requested_by list instead of creating a duplicate.
func (s *Store) CreateApprovalRequest(ctx context.Context, req *ApprovalRequest) (*ApprovalRequest, error) {
	// Try to insert; on conflict update also_requested_by
	var alsoJSON []byte
	err := s.conn().QueryRowContext(ctx, `
		INSERT INTO sandbox.approval_requests (tenant_id, user_id, skill_name, skill_version, source, source_url, approval_scope)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), 'marketplace'), $6, COALESCE(NULLIF($7, ''), 'global'))
		ON CONFLICT (tenant_id, skill_name, skill_version)
		DO UPDATE SET also_requested_by = (
			CASE
				WHEN NOT (sandbox.approval_requests.also_requested_by @> to_jsonb($2::text))
				THEN sandbox.approval_requests.also_requested_by || to_jsonb($2::text)
				ELSE sandbox.approval_requests.also_requested_by
			END
		)
		RETURNING id, tenant_id, user_id, skill_name, skill_version, status, also_requested_by, reviewed_by, review_comment, scan_result, source, source_url, approval_scope, created_at, reviewed_at
	`, req.TenantID, req.UserID, req.SkillName, req.SkillVersion, req.Source, req.SourceURL, req.ApprovalScope).Scan(
		&req.ID, &req.TenantID, &req.UserID, &req.SkillName, &req.SkillVersion,
		&req.Status, &alsoJSON, &req.ReviewedBy, &req.ReviewComment, &req.ScanResult,
		&req.Source, &req.SourceURL, &req.ApprovalScope,
		&req.CreatedAt, &req.ReviewedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create approval request: %w", err)
	}
	if alsoJSON != nil {
		_ = json.Unmarshal(alsoJSON, &req.AlsoRequestedBy)
	}
	return req, nil
}

// ListApprovalRequests returns approval requests for a tenant, optionally filtered by status.
func (s *Store) ListApprovalRequests(ctx context.Context, tenantID, status string) ([]*ApprovalRequest, error) {
	query := `
		SELECT id, tenant_id, user_id, skill_name, skill_version, status, also_requested_by, reviewed_by, review_comment, scan_result, source, source_url, approval_scope, created_at, reviewed_at
		FROM sandbox.approval_requests
		WHERE tenant_id = $1
	`
	args := []any{tenantID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list approval requests: %w", err)
	}
	defer rows.Close()

	var results []*ApprovalRequest
	for rows.Next() {
		var r ApprovalRequest
		var alsoJSON []byte
		if err := rows.Scan(
			&r.ID, &r.TenantID, &r.UserID, &r.SkillName, &r.SkillVersion,
			&r.Status, &alsoJSON, &r.ReviewedBy, &r.ReviewComment, &r.ScanResult,
			&r.Source, &r.SourceURL, &r.ApprovalScope,
			&r.CreatedAt, &r.ReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("scan approval request: %w", err)
		}
		if alsoJSON != nil {
			_ = json.Unmarshal(alsoJSON, &r.AlsoRequestedBy)
		}
		results = append(results, &r)
	}
	return results, rows.Err()
}

// GetApprovalRequest retrieves a single approval request by ID.
func (s *Store) GetApprovalRequest(ctx context.Context, id string) (*ApprovalRequest, error) {
	var r ApprovalRequest
	var alsoJSON []byte
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, skill_name, skill_version, status, also_requested_by, reviewed_by, review_comment, scan_result, source, source_url, approval_scope, created_at, reviewed_at
		FROM sandbox.approval_requests WHERE id = $1
	`, id).Scan(
		&r.ID, &r.TenantID, &r.UserID, &r.SkillName, &r.SkillVersion,
		&r.Status, &alsoJSON, &r.ReviewedBy, &r.ReviewComment, &r.ScanResult,
		&r.Source, &r.SourceURL, &r.ApprovalScope,
		&r.CreatedAt, &r.ReviewedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get approval request: %w", err)
	}
	if alsoJSON != nil {
		_ = json.Unmarshal(alsoJSON, &r.AlsoRequestedBy)
	}
	return &r, nil
}

// UpdateApprovalStatus updates the status of an approval request (approve/reject).
func (s *Store) UpdateApprovalStatus(ctx context.Context, id, status, reviewerID, comment string) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.approval_requests
		SET status = $1, reviewed_by = $2, review_comment = $3, reviewed_at = NOW()
		WHERE id = $4
	`, status, reviewerID, comment, id)
	if err != nil {
		return fmt.Errorf("update approval status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// IsSkillApprovedForTenant checks if a skill has been approved for a tenant.
func (s *Store) IsSkillApprovedForTenant(ctx context.Context, tenantID, skillName string) (bool, error) {
	var exists bool
	err := s.conn().QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sandbox.tenant_approved_skills
			WHERE tenant_id = $1 AND skill_name = $2
		)
	`, tenantID, skillName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check skill approval: %w", err)
	}
	return exists, nil
}

// ApproveSkillForTenant adds a skill to the tenant's approved list.
func (s *Store) ApproveSkillForTenant(ctx context.Context, tenantID, skillName, version, approvedBy string) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sandbox.tenant_approved_skills (tenant_id, skill_name, skill_version, approved_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, skill_name, skill_version) DO NOTHING
	`, tenantID, skillName, version, approvedBy)
	if err != nil {
		return fmt.Errorf("approve skill for tenant: %w", err)
	}
	return nil
}
