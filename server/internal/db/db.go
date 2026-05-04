package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func Open(dsn string) (*sqlx.DB, error) {
	return open(dsn, false)
}

func OpenForMigration(dsn string) (*sqlx.DB, error) {
	return open(dsn, true)
}

func open(dsn string, multiStatements bool) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", dsnWithParams(dsn, multiStatements))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func dsnWithParams(dsn string, multiStatements bool) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%sparseTime=true&charset=utf8mb4&multiStatements=%t", dsn, sep, multiStatements)
}

// MigrateDir applies all .sql files in dir in sorted order, skipping already-applied ones.
// Tracks applied migrations in schema_migrations table.
func MigrateDir(db *sqlx.DB, dir string) error {
	// Ensure tracking table exists
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		var count int
		db.Get(&count, "SELECT COUNT(*) FROM schema_migrations WHERE filename=?", name)
		if count > 0 {
			continue // already applied
		}

		sql, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if _, err := db.Exec(string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}

		db.Exec("INSERT INTO schema_migrations (filename) VALUES (?)", name)
		fmt.Printf("applied migration: %s\n", name)
	}
	return nil
}
