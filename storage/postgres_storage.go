package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStorage struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

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
		pool:         pool,
		queryTimeout: envDuration("DB_QUERY_TIMEOUT_SECONDS", 5*time.Second),
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

// MigratePostgres applies embedded, versioned migrations and exits without
// creating the application store. Production can use a dedicated DB role.
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

func (s *PostgresStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStorage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresStorage) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.queryTimeout)
}

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

func envPositiveInt(name string, fallback int) int {
	value := envInt(name, fallback)
	if value <= 0 {
		return fallback
	}
	return value
}

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

func generateID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
