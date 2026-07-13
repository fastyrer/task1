package storage

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID int64 = 7420198463521

//go:embed migrations/*.up.sql
var migrationFS embed.FS

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	const lockMigrationsQuery = `SELECT pg_advisory_xact_lock($1)`
	const createMigrationsTableQuery = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`
	const migrationAppliedQuery = `
		SELECT EXISTS (
			SELECT 1 FROM schema_migrations WHERE version = $1
		)
	`
	const recordMigrationQuery = `
		INSERT INTO schema_migrations (version, name)
		VALUES ($1, $2)
	`

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migrations transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, lockMigrationsQuery, migrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, createMigrationsTableQuery); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}

		var applied bool
		if err := tx.QueryRow(ctx, migrationAppliedQuery, version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		script, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, recordMigrationQuery, version, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func migrationVersion(name string) (int64, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("invalid migration filename %q", name)
	}

	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("invalid migration version in %q", name)
	}
	return version, nil
}

func verifyMigrationVersion(ctx context.Context, pool *pgxpool.Pool) error {
	const currentMigrationQuery = `
		SELECT COALESCE(max(version), 0)
		FROM schema_migrations
	`

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var expected int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		if version > expected {
			expected = version
		}
	}

	var current int64
	if err := pool.QueryRow(ctx, currentMigrationQuery).Scan(&current); err != nil {
		return fmt.Errorf("read schema migration version (run server migrate first): %w", err)
	}
	if current != expected {
		return fmt.Errorf("database schema version is %d, application requires %d", current, expected)
	}
	return nil
}
