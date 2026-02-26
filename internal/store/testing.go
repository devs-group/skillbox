package store

import "database/sql"

// NewWithDB creates a Store that wraps the given *sql.DB without
// running migrations. This is intended for use in tests that provide
// a mock or in-memory database.
func NewWithDB(db *sql.DB) *Store {
	return &Store{db: db}
}
