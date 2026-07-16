// postgres_storage.go - подключение к PostgreSQL, настройка пула,
// таймаутов и отдельный запуск миграций.
package storage

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStorage - единственная runtime-реализация Store.
// pool переиспользует соединения, а отдельный importTimeout даёт атомарному
// импорту больше времени, чем коротким повседневным запросам.
type PostgresStorage struct {
	pool          *pgxpool.Pool
	queryTimeout  time.Duration
	importTimeout time.Duration
}

// NewPostgresStorage - создаёт готовое PostgreSQL-хранилище.
//
// Порядок инициализации:
//  1. Разбирает DATABASE_URL и применяет настройки пула.
//  2. Ограничивает подключение таймаутом.
//  3. Проверяет реальную связь с БД через Ping.
//  4. Сверяет версию схемы с миграциями в бинарном файле.
func NewPostgresStorage(ctx context.Context, databaseURL string) (*PostgresStorage, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	applyPoolEnv(config)
	config.ConnConfig.RuntimeParams["application_name"] = "task1"

	connectCtx, cancel := context.WithTimeout(
		ctx,
		envDuration("DB_CONNECT_TIMEOUT_SECONDS", 10*time.Second),
	)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	store := &PostgresStorage{
		pool:          pool,
		queryTimeout:  envDuration("DB_QUERY_TIMEOUT_SECONDS", 5*time.Second),
		importTimeout: envDuration("DB_IMPORT_TIMEOUT_SECONDS", 45*time.Second),
	}
	if err := store.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := verifyMigrationVersion(connectCtx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return store, nil
}

// MigratePostgres - применяет встроенные версионированные миграции.
// Для мигратора создаётся отдельный пул из одного соединения.
// В production это позволяет запускать DDL от отдельной роли БД.
func MigratePostgres(ctx context.Context, databaseURL string) error {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse migration DATABASE_URL: %w", err)
	}
	config.MaxConns = 1
	config.MinConns = 0
	config.ConnConfig.RuntimeParams["application_name"] = "task1-migrations"

	migrationCtx, cancel := context.WithTimeout(
		ctx,
		envDuration("DB_MIGRATION_TIMEOUT_SECONDS", 60*time.Second),
	)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(migrationCtx, config)
	if err != nil {
		return fmt.Errorf("create migration pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(migrationCtx); err != nil {
		return fmt.Errorf("ping postgres for migrations: %w", err)
	}
	if err := runMigrations(migrationCtx, pool); err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	return nil
}

// RollbackPostgresMigration - откатывает последнюю применённую миграцию.
// Команда предназначена для контролируемого отката и может удалять данные.
func RollbackPostgresMigration(ctx context.Context, databaseURL string) error {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse migration DATABASE_URL: %w", err)
	}
	config.MaxConns = 1
	config.MinConns = 0
	config.ConnConfig.RuntimeParams["application_name"] = "task1-migrations-down"

	migrationCtx, cancel := context.WithTimeout(
		ctx,
		envDuration("DB_MIGRATION_TIMEOUT_SECONDS", 60*time.Second),
	)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(migrationCtx, config)
	if err != nil {
		return fmt.Errorf("create migration rollback pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(migrationCtx); err != nil {
		return fmt.Errorf("ping postgres for migration rollback: %w", err)
	}
	if err := runMigrationDown(migrationCtx, pool); err != nil {
		return fmt.Errorf("roll back postgres migration: %w", err)
	}
	return nil
}

// Ping - проверяет, что пул может выполнить запрос к PostgreSQL.
func (s *PostgresStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close - корректно закрывает все соединения пула при остановке.
func (s *PostgresStorage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// withTimeout - создаёт контекст с единым таймаутом для операций хранилища.
func (s *PostgresStorage) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.queryTimeout)
}

// withImportTimeout ограничивает тяжёлый preview/commit независимо от коротких CRUD-запросов.
func (s *PostgresStorage) withImportTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.importTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.importTimeout)
}

// applyPoolEnv - применяет к пулу лимиты соединений и времени из env.
func applyPoolEnv(config *pgxpool.Config) {
	config.MaxConns = int32(envPositiveInt("DB_MAX_CONNS", 10))
	config.MinConns = int32(envInt("DB_MIN_CONNS", 0))
	if config.MinConns > config.MaxConns {
		config.MinConns = config.MaxConns
	}
	config.MaxConnLifetime = envDuration("DB_MAX_CONN_LIFETIME_SECONDS", time.Hour)
	config.MaxConnIdleTime = envDuration("DB_MAX_CONN_IDLE_TIME_SECONDS", 30*time.Minute)
	config.HealthCheckPeriod = envDuration("DB_HEALTH_CHECK_SECONDS", 30*time.Second)
}

// envPositiveInt - читает строго положительное целое или возвращает fallback.
func envPositiveInt(name string, fallback int) int {
	value := envInt(name, fallback)
	if value <= 0 {
		return fallback
	}
	return value
}

// envInt - безопасно читает неотрицательное целое из окружения.
func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

// envDuration - преобразует число секунд из env в time.Duration.
func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}
