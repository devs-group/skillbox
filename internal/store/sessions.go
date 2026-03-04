package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session represents a row in the sandbox.sessions table.
type Session struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	ExternalID     string    `json:"external_id"`
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

// GetOrCreateSession finds an existing session by tenant and external ID,
// or creates a new one if none exists. The returned session always has its
// LastAccessedAt updated to now.
func (s *Store) GetOrCreateSession(ctx context.Context, tenantID, externalID string) (*Session, error) {
	sess := &Session{}
	err := s.conn().QueryRowContext(ctx, `
		INSERT INTO sandbox.sessions (tenant_id, external_id)
		VALUES ($1, $2)
		ON CONFLICT (tenant_id, external_id)
		DO UPDATE SET last_accessed_at = NOW()
		RETURNING id, tenant_id, external_id, created_at, last_accessed_at
	`, tenantID, externalID).Scan(
		&sess.ID, &sess.TenantID, &sess.ExternalID,
		&sess.CreatedAt, &sess.LastAccessedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}
	return sess, nil
}

// TouchSession updates the last_accessed_at timestamp for a session.
func (s *Store) TouchSession(ctx context.Context, sessionID string) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE sandbox.sessions SET last_accessed_at = NOW() WHERE id = $1
	`, sessionID)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("touch session rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSession retrieves a session by its internal UUID, scoped to a tenant.
func (s *Store) GetSession(ctx context.Context, tenantID, sessionID string) (*Session, error) {
	sess := &Session{}
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, tenant_id, external_id, created_at, last_accessed_at
		FROM sandbox.sessions
		WHERE id = $1 AND tenant_id = $2
	`, sessionID, tenantID).Scan(
		&sess.ID, &sess.TenantID, &sess.ExternalID,
		&sess.CreatedAt, &sess.LastAccessedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

// GetSessionByExternalID retrieves a session by its external ID (VectorChat session UUID).
func (s *Store) GetSessionByExternalID(ctx context.Context, tenantID, externalID string) (*Session, error) {
	sess := &Session{}
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, tenant_id, external_id, created_at, last_accessed_at
		FROM sandbox.sessions
		WHERE tenant_id = $1 AND external_id = $2
	`, tenantID, externalID).Scan(
		&sess.ID, &sess.TenantID, &sess.ExternalID,
		&sess.CreatedAt, &sess.LastAccessedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by external id: %w", err)
	}
	return sess, nil
}

// ListSessionFiles returns files linked to a session via the session_id
// column on sandbox.files.
func (s *Store) ListSessionFiles(ctx context.Context, tenantID, sessionID string) ([]*File, error) {
	return s.ListFiles(ctx, FileFilter{
		TenantID:  tenantID,
		SessionID: sessionID,
		Limit:     200,
	})
}

// DeleteSession removes a session record. This does NOT delete associated files —
// callers must clean those up separately.
func (s *Store) DeleteSession(ctx context.Context, tenantID, externalID string) error {
	res, err := s.conn().ExecContext(ctx, `
		DELETE FROM sandbox.sessions WHERE tenant_id = $1 AND external_id = $2
	`, tenantID, externalID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete session rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
