package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// File represents a row in the sandbox.files table.
type File struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	SessionID   string    `json:"session_id,omitempty"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	S3Key       string    `json:"s3_key"`
	Version     int       `json:"version"`
	ParentID    *string   `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FileFilter describes the filter and pagination parameters for listing files.
type FileFilter struct {
	TenantID    string
	SessionID   string
	ExecutionID string
	Limit       int
	Offset      int
}

// CreateFile inserts a new file record. The File is mutated in place with
// the server-generated ID and timestamps.
func (s *Store) CreateFile(ctx context.Context, f *File) (*File, error) {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO sandbox.files (tenant_id, session_id, execution_id, name, content_type, size_bytes, s3_key, version, parent_id)
		VALUES ($1, nullif($2, ''), nullif($3, '')::UUID, $4, $5, $6, $7, $8, nullif($9, '')::UUID)
		RETURNING id, created_at, updated_at
	`, f.TenantID, f.SessionID, f.ExecutionID, f.Name, f.ContentType,
		f.SizeBytes, f.S3Key, f.Version, nilIfEmpty(f.ParentID),
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	return f, nil
}

// GetFile retrieves a single file record by its UUID.
// Returns ErrNotFound if the file does not exist.
func (s *Store) GetFile(ctx context.Context, id string) (*File, error) {
	f := &File{}
	var sessionID, executionID, parentID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, session_id, execution_id, name, content_type,
		       size_bytes, s3_key, version, parent_id, created_at, updated_at
		FROM sandbox.files
		WHERE id = $1
	`, id).Scan(
		&f.ID, &f.TenantID, &sessionID, &executionID, &f.Name, &f.ContentType,
		&f.SizeBytes, &f.S3Key, &f.Version, &parentID, &f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	if sessionID.Valid {
		f.SessionID = sessionID.String
	}
	if executionID.Valid {
		f.ExecutionID = executionID.String
	}
	if parentID.Valid {
		f.ParentID = &parentID.String
	}
	return f, nil
}

// ListFiles returns files matching the given filter, ordered by creation
// time (newest first), with pagination via limit and offset.
func (s *Store) ListFiles(ctx context.Context, filter FileFilter) ([]*File, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, session_id, execution_id, name, content_type,
		       size_bytes, s3_key, version, parent_id, created_at, updated_at
		FROM sandbox.files
		WHERE tenant_id = $1
		  AND ($2 = '' OR session_id = $2)
		  AND ($3 = '' OR execution_id::TEXT = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`, filter.TenantID, filter.SessionID, filter.ExecutionID,
		filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		var sessionID, executionID, parentID sql.NullString
		if err := rows.Scan(
			&f.ID, &f.TenantID, &sessionID, &executionID, &f.Name, &f.ContentType,
			&f.SizeBytes, &f.S3Key, &f.Version, &parentID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan file row: %w", err)
		}
		if sessionID.Valid {
			f.SessionID = sessionID.String
		}
		if executionID.Valid {
			f.ExecutionID = executionID.String
		}
		if parentID.Valid {
			f.ParentID = &parentID.String
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file rows: %w", err)
	}

	return files, nil
}

// UpdateFile updates a file record's mutable fields: name, content_type,
// size_bytes, s3_key, and version. It also sets updated_at to now().
func (s *Store) UpdateFile(ctx context.Context, f *File) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sandbox.files
		SET name = $2,
		    content_type = $3,
		    size_bytes = $4,
		    s3_key = $5,
		    version = $6,
		    parent_id = nullif($7, '')::UUID,
		    updated_at = NOW()
		WHERE id = $1
	`, f.ID, f.Name, f.ContentType, f.SizeBytes, f.S3Key,
		f.Version, nilIfEmpty(f.ParentID),
	)
	if err != nil {
		return fmt.Errorf("update file: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update file rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteFile removes a file record by its UUID.
func (s *Store) DeleteFile(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM sandbox.files WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete file rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// ListFileVersions returns all versions of a file, following the parent_id
// chain. It finds all files where id = fileID or parent_id = fileID,
// ordered by version descending.
func (s *Store) ListFileVersions(ctx context.Context, fileID string) ([]*File, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, session_id, execution_id, name, content_type,
		       size_bytes, s3_key, version, parent_id, created_at, updated_at
		FROM sandbox.files
		WHERE id = $1 OR parent_id = $1
		ORDER BY version DESC
	`, fileID)
	if err != nil {
		return nil, fmt.Errorf("list file versions: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		var sessionID, executionID, parentID sql.NullString
		if err := rows.Scan(
			&f.ID, &f.TenantID, &sessionID, &executionID, &f.Name, &f.ContentType,
			&f.SizeBytes, &f.S3Key, &f.Version, &parentID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan file version row: %w", err)
		}
		if sessionID.Valid {
			f.SessionID = sessionID.String
		}
		if executionID.Valid {
			f.ExecutionID = executionID.String
		}
		if parentID.Valid {
			f.ParentID = &parentID.String
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file version rows: %w", err)
	}

	return files, nil
}

// nilIfEmpty returns "" if the pointer is nil, otherwise returns the string
// value. Used for nullable text columns that should store NULL for empty.
func nilIfEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
