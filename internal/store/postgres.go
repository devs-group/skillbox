package store

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps a PostgreSQL connection pool and provides data-access methods
// for the Skillbox runtime.
type Store struct {
	db *sql.DB
}

// New opens a connection pool to PostgreSQL using the provided DSN,
// verifies connectivity, and runs all embedded migrations. It returns
// a ready-to-use Store or an error if any step fails.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return s, nil
}

// Close releases the database connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use in tests or advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// migrate creates a migration tracking table (if it doesn't exist) and
// applies any embedded SQL migrations that have not yet been recorded.
// Migrations run inside individual transactions so a partial failure
// leaves the database in a known state.
func (s *Store) migrate() error {
	// Ensure the migration tracking table exists.
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sandbox_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Check whether this migration has already been applied.
		var exists bool
		err := s.db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM sandbox_migrations WHERE filename = $1)",
			entry.Name(),
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}
		if exists {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(
			"INSERT INTO sandbox_migrations (filename) VALUES ($1)",
			entry.Name(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
