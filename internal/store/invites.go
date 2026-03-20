package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// InviteCode represents a row in the sandbox.invite_codes table.
type InviteCode struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	Code      string     `json:"code"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UsedBy    *string    `json:"used_by,omitempty"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// GenerateInviteCode generates a cryptographically random invite code.
func GenerateInviteCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateInviteCode creates a new invite code for a tenant.
func (s *Store) CreateInviteCode(ctx context.Context, tenantID string, createdBy *string) (*InviteCode, error) {
	code, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}

	inv := &InviteCode{
		TenantID:  tenantID,
		Code:      code,
		CreatedBy: createdBy,
	}

	err = s.conn().QueryRowContext(ctx, `
		INSERT INTO sandbox.invite_codes (tenant_id, code, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, inv.TenantID, inv.Code, inv.CreatedBy).Scan(&inv.ID, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create invite code: %w", err)
	}
	return inv, nil
}

// RedeemInviteCode marks an invite code as used by the given user ID.
// Returns the invite code record (with tenant_id) or ErrNotFound if the
// code is invalid, already used, or expired.
func (s *Store) RedeemInviteCode(ctx context.Context, code, userID string) (*InviteCode, error) {
	var inv InviteCode
	err := s.conn().QueryRowContext(ctx, `
		UPDATE sandbox.invite_codes
		SET used_by = $1, used_at = NOW()
		WHERE code = $2
		  AND used_by IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING id, tenant_id, code, created_by, used_by, used_at, expires_at, created_at
	`, userID, code).Scan(
		&inv.ID, &inv.TenantID, &inv.Code, &inv.CreatedBy,
		&inv.UsedBy, &inv.UsedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("redeem invite code: %w", err)
	}
	return &inv, nil
}

// ListInviteCodes returns all invite codes for a tenant.
func (s *Store) ListInviteCodes(ctx context.Context, tenantID string) ([]*InviteCode, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, tenant_id, code, created_by, used_by, used_at, expires_at, created_at
		FROM sandbox.invite_codes
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list invite codes: %w", err)
	}
	defer rows.Close()

	var codes []*InviteCode
	for rows.Next() {
		var inv InviteCode
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.Code, &inv.CreatedBy, &inv.UsedBy, &inv.UsedAt, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invite code: %w", err)
		}
		codes = append(codes, &inv)
	}
	return codes, rows.Err()
}

// CountUsersInTenant returns the number of users in a tenant.
// Used to determine if the first user should get admin role.
func (s *Store) CountUsersInTenant(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := s.conn().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sandbox.users WHERE tenant_id = $1
	`, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users in tenant: %w", err)
	}
	return count, nil
}
