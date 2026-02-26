package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// APIKey represents a row in the sandbox.api_keys table.
type APIKey struct {
	ID        string
	KeyHash   string
	TenantID  string
	Name      string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ValidateKey looks up an API key by its SHA-256 hash. It returns the
// matching APIKey if the hash exists and the key has not been revoked.
// A nil APIKey (with nil error) is returned when the key is not found
// or has been revoked.
func (s *Store) ValidateKey(ctx context.Context, keyHash string) (*APIKey, error) {
	k := &APIKey{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, key_hash, tenant_id, name, created_at, revoked_at
		   FROM sandbox.api_keys
		  WHERE key_hash = $1`,
		keyHash,
	).Scan(&k.ID, &k.KeyHash, &k.TenantID, &k.Name, &k.CreatedAt, &k.RevokedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("validate key: %w", err)
	}

	// Treat revoked keys as invalid.
	if k.RevokedAt != nil {
		return nil, nil
	}

	return k, nil
}

// CreateKey inserts a new API key record and returns the populated APIKey
// (including the server-generated UUID and timestamp).
func (s *Store) CreateKey(ctx context.Context, tenantID, name, keyHash string) (*APIKey, error) {
	k := &APIKey{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sandbox.api_keys (key_hash, tenant_id, name)
		 VALUES ($1, $2, $3)
		 RETURNING id, key_hash, tenant_id, name, created_at, revoked_at`,
		keyHash, tenantID, name,
	).Scan(&k.ID, &k.KeyHash, &k.TenantID, &k.Name, &k.CreatedAt, &k.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("create key: %w", err)
	}
	return k, nil
}

// ListKeys returns all API keys belonging to the given tenant, ordered
// by creation time (newest first). Revoked keys are included so the
// caller can display their status.
func (s *Store) ListKeys(ctx context.Context, tenantID string) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, key_hash, tenant_id, name, created_at, revoked_at
		   FROM sandbox.api_keys
		  WHERE tenant_id = $1
		  ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.TenantID, &k.Name, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan key row: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key rows: %w", err)
	}

	return keys, nil
}

// RevokeKey soft-deletes an API key by setting its revoked_at timestamp.
// It returns an error if the key does not exist or is already revoked.
func (s *Store) RevokeKey(ctx context.Context, keyID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sandbox.api_keys
		    SET revoked_at = now()
		  WHERE id = $1 AND revoked_at IS NULL`,
		keyID,
	)
	if err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke key rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("key %s not found or already revoked", keyID)
	}

	return nil
}

// GetAPIKeyByHash looks up an API key by its SHA-256 hex hash.
// It returns ErrNotFound if no matching key exists.
func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	k := &APIKey{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, key_hash, tenant_id, name, created_at, revoked_at
		FROM sandbox.api_keys
		WHERE key_hash = $1
	`, hash).Scan(&k.ID, &k.KeyHash, &k.TenantID, &k.Name, &k.CreatedAt, &k.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
