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

// migrationFS содержит парные up/down SQL-файлы внутри Go-бинарника.
//
//go:embed migrations/*.up.sql migrations/*.down.sql
var migrationFS embed.FS

type embeddedMigration struct {
	Version    int64
	Name       string
	Script     []byte
	DownName   string
	DownScript []byte
	Checksum   string
}

// runMigrationDown откатывает последнюю применённую миграцию.
func runMigrationDown(ctx context.Context, pool *pgxpool.Pool) error {
	const lockMigrationsQuery = `SELECT pg_advisory_xact_lock($1)`
	const migrationsTableExistsQuery = `
		SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL
	`
	const latestMigrationQuery = `
		SELECT version, name, checksum
		FROM schema_migrations
		ORDER BY version DESC
		LIMIT 1
	`
	const deleteMigrationQuery = `DELETE FROM schema_migrations WHERE version = $1`
	const remainingMigrationsQuery = `SELECT count(*) FROM schema_migrations`
	const dropMigrationsTableQuery = `DROP TABLE schema_migrations`

	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration rollback transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, lockMigrationsQuery, migrationLockID); err != nil {
		return fmt.Errorf("lock migration rollback: %w", err)
	}

	var migrationsTableExists bool
	if err := tx.QueryRow(ctx, migrationsTableExistsQuery).Scan(&migrationsTableExists); err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if !migrationsTableExists {
		return fmt.Errorf("no applied migrations to roll back")
	}

	var applied embeddedMigration
	if err := tx.QueryRow(ctx, latestMigrationQuery).Scan(&applied.Version, &applied.Name, &applied.Checksum); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no applied migrations to roll back")
		}
		return fmt.Errorf("read latest migration: %w", err)
	}

	var target *embeddedMigration
	for index := range migrations {
		if migrations[index].Version == applied.Version {
			target = &migrations[index]
			break
		}
	}
	if target == nil || target.Name != applied.Name || target.Checksum != applied.Checksum {
		return fmt.Errorf("applied migration %d does not match application", applied.Version)
	}

	if _, err := tx.Exec(ctx, string(target.DownScript)); err != nil {
		return fmt.Errorf("roll back migration %s: %w", target.Name, err)
	}
	if _, err := tx.Exec(ctx, deleteMigrationQuery, target.Version); err != nil {
		return fmt.Errorf("delete migration %s: %w", target.Name, err)
	}

	var remaining int
	if err := tx.QueryRow(ctx, remainingMigrationsQuery).Scan(&remaining); err != nil {
		return fmt.Errorf("count remaining migrations: %w", err)
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, dropMigrationsTableQuery); err != nil {
			return fmt.Errorf("drop empty schema_migrations: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration rollback: %w", err)
	}
	return nil
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
	downNames := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(entry.Name(), ".up.sql"):
			names = append(names, entry.Name())
		case strings.HasSuffix(entry.Name(), ".down.sql"):
			downNames[entry.Name()] = struct{}{}
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
		downName := strings.TrimSuffix(name, ".up.sql") + ".down.sql"
		if _, exists := downNames[downName]; !exists {
			return nil, fmt.Errorf("down migration for %q is missing", name)
		}
		downScript, err := migrationFS.ReadFile("migrations/" + downName)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", downName, err)
		}
		delete(downNames, downName)

		checksum := fmt.Sprintf("%x", sha256.Sum256(script))
		migrations = append(migrations, embeddedMigration{
			Version:    version,
			Name:       name,
			Script:     script,
			DownName:   downName,
			DownScript: downScript,
			Checksum:   checksum,
		})
		seenVersions[version] = name
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no embedded migrations found")
	}
	if len(downNames) > 0 {
		for name := range downNames {
			return nil, fmt.Errorf("up migration for %q is missing", name)
		}
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
