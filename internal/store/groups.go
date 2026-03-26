package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Group represents a row in the sandbox.groups table.
type Group struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalID  *string   `json:"external_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateGroup inserts a new group record.
func (s *Store) CreateGroup(ctx context.Context, g *Group) error {
	err := s.conn().QueryRowContext(ctx, `
		INSERT INTO sandbox.groups (tenant_id, name, description, external_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, g.TenantID, g.Name, g.Description, g.ExternalID).Scan(&g.ID, &g.CreatedAt)
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}
	return nil
}

// ListGroups returns all groups for a tenant.
func (s *Store) ListGroups(ctx context.Context, tenantID string) ([]*Group, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, tenant_id, name, description, external_id, created_at
		FROM sandbox.groups
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var groups []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.ExternalID, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}

// UpdateGroup updates a group's name and description.
func (s *Store) UpdateGroup(ctx context.Context, g *Group) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.groups SET name = $1, description = $2 WHERE id = $3 AND tenant_id = $4
	`, g.Name, g.Description, g.ID, g.TenantID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteGroup removes a group by ID.
func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	res, err := s.conn().ExecContext(ctx, `DELETE FROM sandbox.groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddUserToGroup adds a user to a group.
func (s *Store) AddUserToGroup(ctx context.Context, userID, groupID string) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sandbox.user_groups (user_id, group_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, groupID)
	if err != nil {
		return fmt.Errorf("add user to group: %w", err)
	}
	return nil
}

// RemoveUserFromGroup removes a user from a group.
func (s *Store) RemoveUserFromGroup(ctx context.Context, userID, groupID string) error {
	res, err := s.conn().ExecContext(ctx, `
		DELETE FROM sandbox.user_groups WHERE user_id = $1 AND group_id = $2
	`, userID, groupID)
	if err != nil {
		return fmt.Errorf("remove user from group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGroupMembers returns all users in a group.
func (s *Store) ListGroupMembers(ctx context.Context, groupID string) ([]*User, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT u.id, u.kratos_identity_id, u.tenant_id, u.email, u.display_name, u.role, u.created_at, u.updated_at
		FROM sandbox.users u
		JOIN sandbox.user_groups ug ON ug.user_id = u.id
		WHERE ug.group_id = $1
		ORDER BY u.email
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	defer rows.Close() //nolint:errcheck

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

// GetGroup retrieves a group by ID.
func (s *Store) GetGroup(ctx context.Context, id string) (*Group, error) {
	var g Group
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, tenant_id, name, description, external_id, created_at
		FROM sandbox.groups WHERE id = $1
	`, id).Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.ExternalID, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get group: %w", err)
	}
	return &g, nil
}
