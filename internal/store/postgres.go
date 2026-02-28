package store

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
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
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		_ = db.Close()
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

// migrate runs embedded SQL migrations using goose. On first run after
// switching from the old custom migration system, it seeds goose's version
// table with already-applied migrations so nothing is re-run.
func (s *Store) migrate() error {
	// Seed goose state from the legacy sandbox_migrations table if it exists.
	// This ensures a smooth transition: databases that already ran migrations
	// 001-003 with the old system won't re-apply them.
	if err := s.seedGooseFromLegacy(); err != nil {
		return fmt.Errorf("seed goose from legacy: %w", err)
	}

	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

// seedGooseFromLegacy checks if the old sandbox_migrations table exists and,
// if so, seeds goose's version table with the versions that were already
// applied. This is a one-time bridging step that runs before goose.Up().
func (s *Store) seedGooseFromLegacy() error {
	// Check if the legacy migration table exists.
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'sandbox_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check legacy table: %w", err)
	}
	if !exists {
		return nil // Fresh database, nothing to seed.
	}

	// Check if goose has already been initialised (avoid double-seeding).
	var gooseExists bool
	err = s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'goose_db_version'
		)
	`).Scan(&gooseExists)
	if err != nil {
		return fmt.Errorf("check goose table: %w", err)
	}
	if gooseExists {
		return nil // Goose already initialised, skip seeding.
	}

	// Map legacy filenames to goose version numbers.
	legacyToVersion := map[string]int64{
		"001_initial.sql":              1,
		"002_skills_metadata.sql":      2,
		"003_execution_status_check.sql": 3,
	}

	// Read which migrations were applied in the legacy system.
	rows, err := s.db.Query("SELECT filename FROM sandbox_migrations")
	if err != nil {
		return fmt.Errorf("read legacy migrations: %w", err)
	}
	defer rows.Close()

	var appliedVersions []int64
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return fmt.Errorf("scan legacy row: %w", err)
		}
		if v, ok := legacyToVersion[filename]; ok {
			appliedVersions = append(appliedVersions, v)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy rows: %w", err)
	}

	if len(appliedVersions) == 0 {
		return nil
	}

	// Create goose version table and seed it.
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS goose_db_version (
			id         SERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			is_applied BOOLEAN NOT NULL,
			tstamp     TIMESTAMP DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create goose table: %w", err)
	}

	// Insert initial row (version 0, goose convention).
	_, err = s.db.Exec(`
		INSERT INTO goose_db_version (version_id, is_applied)
		SELECT 0, true
		WHERE NOT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id = 0)
	`)
	if err != nil {
		return fmt.Errorf("seed goose version 0: %w", err)
	}

	for _, v := range appliedVersions {
		_, err = s.db.Exec(`
			INSERT INTO goose_db_version (version_id, is_applied)
			SELECT $1, true
			WHERE NOT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id = $1 AND is_applied = true)
		`, v)
		if err != nil {
			return fmt.Errorf("seed goose version %d: %w", v, err)
		}
	}

	return nil
}
