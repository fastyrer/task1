package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"task1/backend/models"
)

const uploadedFilesSchema = `
CREATE TABLE IF NOT EXISTS uploaded_files (
	id TEXT PRIMARY KEY,
	original_filename TEXT NOT NULL DEFAULT '',
	format TEXT NOT NULL DEFAULT '',
	row_count INTEGER NOT NULL DEFAULT 0,
	column_count INTEGER NOT NULL DEFAULT 0,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS uploaded_files_created_at_idx ON uploaded_files (created_at DESC);
CREATE INDEX IF NOT EXISTS uploaded_files_format_idx ON uploaded_files (format);
`

type PostgresStorage struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewPostgresStorage(ctx context.Context, databaseURL string) (*PostgresStorage, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	applyPoolEnv(config)

	connectCtx, cancel := context.WithTimeout(ctx, envDuration("DB_CONNECT_TIMEOUT_SECONDS", 10*time.Second))
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
	if err := store.migrate(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}

	return store, nil
}

func (s *PostgresStorage) SaveFileData(ctx context.Context, data models.FileData) (string, error) {
	fileID := strings.TrimSpace(data.ID)
	if fileID == "" {
		fileID = generateFileID()
	}
	data.ID = fileID

	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal file data: %w", err)
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err = s.pool.Exec(queryCtx, `
		INSERT INTO uploaded_files (
			id,
			original_filename,
			format,
			row_count,
			column_count,
			payload
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			original_filename = EXCLUDED.original_filename,
			format = EXCLUDED.format,
			row_count = EXCLUDED.row_count,
			column_count = EXCLUDED.column_count,
			payload = EXCLUDED.payload,
			updated_at = now()
	`, fileID, data.OriginalFilename, data.Format, data.Stats.RowCount, data.Stats.ColumnCount, payload)
	if err != nil {
		return "", fmt.Errorf("save file data: %w", err)
	}

	return fileID, nil
}

func (s *PostgresStorage) GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return models.FileData{}, false, nil
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	var payload []byte
	err := s.pool.QueryRow(queryCtx, `
		SELECT payload
		FROM uploaded_files
		WHERE id = $1
	`, fileID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.FileData{}, false, nil
	}
	if err != nil {
		return models.FileData{}, false, fmt.Errorf("get file data: %w", err)
	}

	var data models.FileData
	if err := json.Unmarshal(payload, &data); err != nil {
		return models.FileData{}, false, fmt.Errorf("unmarshal file data: %w", err)
	}
	if data.ID == "" {
		data.ID = fileID
	}

	return data, true, nil
}

func (s *PostgresStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStorage) Close() {
	s.pool.Close()
}

func (s *PostgresStorage) Driver() string {
	return "postgres"
}

func (s *PostgresStorage) migrate(ctx context.Context) error {
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(queryCtx, uploadedFilesSchema)
	return err
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
