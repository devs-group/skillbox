package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Execution represents a row in the sandbox.executions table.
type Execution struct {
	ID           string          `json:"execution_id"`
	SkillName    string          `json:"skill_name"`
	SkillVersion string          `json:"skill_version"`
	TenantID     string          `json:"tenant_id"`
	Status       string          `json:"status"`
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Logs         string          `json:"logs,omitempty"`
	FilesURL     string          `json:"files_url,omitempty"`
	FilesList    []string        `json:"files_list,omitempty"`
	DurationMs   int64           `json:"duration_ms"`
	Error        *string         `json:"error"`
	CreatedAt    time.Time       `json:"created_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
}

// CreateExecution inserts a new execution record with status "running".
// The Execution is mutated in place with the server-generated ID and timestamp.
func (s *Store) CreateExecution(ctx context.Context, e *Execution) (*Execution, error) {
	e.Status = "running"
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO sandbox.executions (skill_name, skill_version, tenant_id, status, input)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, e.SkillName, e.SkillVersion, e.TenantID, e.Status, nullableJSON(e.Input),
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}
	return e, nil
}

// InsertExecution creates a new execution record. This is an alias kept
// for compatibility with callers that set status before calling.
func (s *Store) InsertExecution(ctx context.Context, e *Execution) error {
	return s.db.QueryRowContext(ctx, `
		INSERT INTO sandbox.executions (skill_name, skill_version, tenant_id, status, input)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, e.SkillName, e.SkillVersion, e.TenantID, e.Status, e.Input,
	).Scan(&e.ID, &e.CreatedAt)
}

// UpdateExecution writes back mutable fields for an existing execution.
// Typically called once the execution has completed (or timed out).
func (s *Store) UpdateExecution(ctx context.Context, e *Execution) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sandbox.executions
		SET status = $2,
		    output = $3,
		    logs = $4,
		    files_url = $5,
		    files_list = $6,
		    duration_ms = $7,
		    error = $8,
		    finished_at = $9
		WHERE id = $1 AND status = 'running'
	`, e.ID, e.Status, nullableJSON(e.Output), e.Logs, e.FilesURL,
		pq.Array(e.FilesList), e.DurationMs, e.Error, e.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update execution rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("execution %s not found or already finished", e.ID)
	}

	return nil
}

// GetExecution retrieves a single execution by its UUID.
// Returns ErrNotFound if the execution does not exist.
func (s *Store) GetExecution(ctx context.Context, id string) (*Execution, error) {
	e := &Execution{}
	var filesList []sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, skill_name, skill_version, tenant_id, status,
		       input, output, logs, files_url, files_list,
		       duration_ms, error, created_at, finished_at
		FROM sandbox.executions
		WHERE id = $1
	`, id).Scan(
		&e.ID, &e.SkillName, &e.SkillVersion, &e.TenantID, &e.Status,
		&e.Input, &e.Output, &e.Logs, &e.FilesURL, pq.Array(&filesList),
		&e.DurationMs, &e.Error, &e.CreatedAt, &e.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	e.FilesList = make([]string, 0, len(filesList))
	for _, f := range filesList {
		if f.Valid {
			e.FilesList = append(e.FilesList, f.String)
		}
	}
	return e, nil
}

// ListExecutions returns executions for a tenant ordered by creation time
// (newest first), with pagination via limit and offset.
func (s *Store) ListExecutions(ctx context.Context, tenantID string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, skill_name, skill_version, tenant_id, status,
		       input, output, logs, files_url, files_list,
		       duration_ms, error, created_at, finished_at
		FROM sandbox.executions
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var execs []Execution
	for rows.Next() {
		var e Execution
		var filesList []sql.NullString
		if err := rows.Scan(
			&e.ID, &e.SkillName, &e.SkillVersion, &e.TenantID, &e.Status,
			&e.Input, &e.Output, &e.Logs, &e.FilesURL, pq.Array(&filesList),
			&e.DurationMs, &e.Error, &e.CreatedAt, &e.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan execution row: %w", err)
		}
		e.FilesList = make([]string, 0, len(filesList))
		for _, f := range filesList {
			if f.Valid {
				e.FilesList = append(e.FilesList, f.String)
			}
		}
		execs = append(execs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate execution rows: %w", err)
	}

	return execs, nil
}

// nullableJSON returns nil for empty or null JSON so the database receives
// a proper NULL instead of an empty byte slice.
func nullableJSON(data json.RawMessage) interface{} {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return []byte(data)
}
