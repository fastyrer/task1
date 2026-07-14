// migrations.go - запуск встроенных SQL-миграций и проверка
// совместимости схемы БД с текущим бинарным файлом.
package storage

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationLockID - стабильный ключ PostgreSQL advisory lock для этого проекта.
// Он не даёт двум экземплярам мигратора менять схему одновременно.
const migrationLockID int64 = 7420198463521

// migrationFS содержит .up.sql-файлы внутри Go-бинарника.
//
//go:embed migrations/*.up.sql
var migrationFS embed.FS

type embeddedMigration struct {
	Version  int64
	Name     string
	Script   []byte
	Checksum string
}

type migrationQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// runMigrations - применяет ещё не выполненные миграции по возрастанию версии.
//
// Алгоритм:
//  1. Находит и сортирует встроенные .up.sql-файлы.
//  2. Открывает общую транзакцию и берёт advisory lock.
//  3. Создаёт таблицу schema_migrations, если её ещё нет.
//  4. Пропускает уже зафиксированные версии.
//  5. Выполняет SQL и записывает версию в schema_migrations.
//  6. Коммитит всю серию; при ошибке defer выполнит rollback.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Запросы объявлены рядом с логикой метода, чтобы их было видно при чтении.
	const lockMigrationsQuery = `SELECT pg_advisory_xact_lock($1)`
	const createMigrationsTableQuery = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`
	const migrationAppliedQuery = `SELECT name, checksum FROM schema_migrations WHERE version = $1`
	const recordMigrationQuery = `
		INSERT INTO schema_migrations (version, name, checksum)
		VALUES ($1, $2, $3)
	`

	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}

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

	for _, migration := range migrations {
		var appliedName, appliedChecksum string
		err := tx.QueryRow(ctx, migrationAppliedQuery, migration.Version).Scan(&appliedName, &appliedChecksum)
		switch {
		case err == nil:
			if appliedName != migration.Name || appliedChecksum != migration.Checksum {
				return fmt.Errorf("migration %d differs from applied schema", migration.Version)
			}
			continue
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("check migration %s: %w", migration.Name, err)
		}

		if _, err := tx.Exec(ctx, string(migration.Script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
		if _, err := tx.Exec(ctx, recordMigrationQuery, migration.Version, migration.Name, migration.Checksum); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
	}
	if err := verifyAppliedMigrations(ctx, tx, migrations); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

// loadEmbeddedMigrations читает, сортирует и хеширует встроенные up-миграции.
func loadEmbeddedMigrations() ([]embeddedMigration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	migrations := make([]embeddedMigration, 0, len(names))
	seenVersions := make(map[int64]string, len(names))
	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return nil, err
		}
		if previous, exists := seenVersions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, name)
		}

		script, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(script))
		migrations = append(migrations, embeddedMigration{
			Version: version, Name: name, Script: script, Checksum: checksum,
		})
		seenVersions[version] = name
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no embedded migrations found")
	}
	return migrations, nil
}

// migrationVersion - извлекает числовую версию из имени вида 000001_name.up.sql.
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

// verifyMigrationVersion проверяет точное совпадение миграций БД и бинарника.
func verifyMigrationVersion(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}
	return verifyAppliedMigrations(ctx, pool, migrations)
}

func verifyAppliedMigrations(ctx context.Context, queryer migrationQueryer, expected []embeddedMigration) error {
	const appliedMigrationsQuery = `
		SELECT version, name, checksum
		FROM schema_migrations
		ORDER BY version
	`

	rows, err := queryer.Query(ctx, appliedMigrationsQuery)
	if err != nil {
		return fmt.Errorf("read schema migrations (run server migrate first): %w", err)
	}
	defer rows.Close()

	applied := make([]embeddedMigration, 0, len(expected))
	for rows.Next() {
		var migration embeddedMigration
		if err := rows.Scan(&migration.Version, &migration.Name, &migration.Checksum); err != nil {
			return fmt.Errorf("scan schema migration: %w", err)
		}
		applied = append(applied, migration)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema migrations: %w", err)
	}

	if len(applied) != len(expected) {
		return fmt.Errorf("database has %d migrations, application requires %d", len(applied), len(expected))
	}
	for index := range expected {
		if applied[index].Version != expected[index].Version ||
			applied[index].Name != expected[index].Name ||
			applied[index].Checksum != expected[index].Checksum {
			return fmt.Errorf("database migration %d does not match application", expected[index].Version)
		}
	}
	return nil
}
