package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User represents a row in the sandbox.users table.
type User struct {
	ID               string    `json:"id"`
	KratosIdentityID string    `json:"kratos_identity_id"`
	TenantID         string    `json:"tenant_id"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GetUserByKratosID retrieves a user by their Ory Kratos identity ID.
func (s *Store) GetUserByKratosID(ctx context.Context, kratosID string) (*User, error) {
	var u User
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, kratos_identity_id, tenant_id, email, display_name, role, created_at, updated_at
		FROM sandbox.users
		WHERE kratos_identity_id = $1
	`, kratosID).Scan(&u.ID, &u.KratosIdentityID, &u.TenantID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by kratos id: %w", err)
	}
	return &u, nil
}

// GetUserByID retrieves a user by their Skillbox user ID (UUID string).
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, kratos_identity_id, tenant_id, email, display_name, role, created_at, updated_at
		FROM sandbox.users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.KratosIdentityID, &u.TenantID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// ListUsers returns all users for a tenant.
func (s *Store) ListUsers(ctx context.Context, tenantID string) ([]*User, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, kratos_identity_id, tenant_id, email, display_name, role, created_at, updated_at
		FROM sandbox.users
		WHERE tenant_id = $1
		ORDER BY email
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.KratosIdentityID, &u.TenantID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

// CreateUser inserts a new user record.
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	err := s.conn().QueryRowContext(ctx, `
		INSERT INTO sandbox.users (kratos_identity_id, tenant_id, email, display_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`, u.KratosIdentityID, u.TenantID, u.Email, u.DisplayName, u.Role).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetOrCreateUser finds a user by Kratos ID, or creates one if not found.
// Used during first login to auto-provision the user record.
func (s *Store) GetOrCreateUser(ctx context.Context, kratosID, tenantID, email, displayName, role string) (*User, error) {
	u, err := s.GetUserByKratosID(ctx, kratosID)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	newUser := &User{
		KratosIdentityID: kratosID,
		TenantID:         tenantID,
		Email:            email,
		DisplayName:      displayName,
		Role:             role,
	}
	if err := s.CreateUser(ctx, newUser); err != nil {
		return nil, err
	}
	return newUser, nil
}

// UpdateUserRole updates the role for a given user.
func (s *Store) UpdateUserRole(ctx context.Context, id string, role string) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.users SET role = $1, updated_at = NOW() WHERE id = $2
	`, role, id)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
