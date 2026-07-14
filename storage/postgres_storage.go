// postgres_storage.go - подключение к PostgreSQL, настройка пула,
// таймаутов и отдельный запуск миграций.
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"task1/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

CREATE TABLE IF NOT EXISTS contacts (
	id TEXT PRIMARY KEY,
	phone TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	discount TEXT NOT NULL DEFAULT '',
	file_id TEXT NOT NULL REFERENCES uploaded_files(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS contacts_file_id_idx ON contacts (file_id);
CREATE INDEX IF NOT EXISTS contacts_email_idx ON contacts (email);

CREATE TABLE IF NOT EXISTS contact_versions (
	id SERIAL PRIMARY KEY,
	contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
	phone TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	discount TEXT NOT NULL DEFAULT '',
	file_id TEXT NOT NULL,
	action TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS contact_versions_contact_id_idx ON contact_versions (contact_id);
`

type PostgresStorage struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
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

	_, err = s.pool.Exec(queryCtx, `
		INSERT INTO uploaded_files (
			id, original_filename, format, row_count, column_count, payload
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

func (s *PostgresStorage) SaveContact(ctx context.Context, contact models.Contact) (string, error) {
	contactID := strings.TrimSpace(contact.ID)
	if contactID == "" {
		contactID = generateFileID()
	}
	contact.ID = contactID

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(queryCtx, `
		INSERT INTO contacts (id, phone, email, name, discount, file_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			phone = EXCLUDED.phone,
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			discount = EXCLUDED.discount,
			file_id = EXCLUDED.file_id,
			updated_at = now()
	`, contactID, contact.Phone, contact.Email, contact.Name, contact.Discount, contact.FileID)
	if err != nil {
		return "", fmt.Errorf("save contact: %w", err)
	}

	s.saveContactVersion(ctx, contact, "created")

	return contactID, nil
}

func (s *PostgresStorage) GetContactByPhone(ctx context.Context, phone string) (models.Contact, bool, error) {
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	c, err := s.scanContact(queryCtx, `WHERE phone = $1`, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Contact{}, false, nil
	}
	if err != nil {
		return models.Contact{}, false, fmt.Errorf("get contact by phone: %w", err)
	}

	return c, true, nil
}

func (s *PostgresStorage) ListContactsByFileID(ctx context.Context, fileID string) ([]models.Contact, error) {
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, `
		SELECT id, phone, email, name, discount, file_id, created_at, updated_at
		FROM contacts
		WHERE file_id = $1
		ORDER BY created_at
	`, fileID)
	if err != nil {
		return nil, fmt.Errorf("list contacts by file id: %w", err)
	}
	defer rows.Close()

	contacts := make([]models.Contact, 0)
	for rows.Next() {
		var c models.Contact
		if err := rows.Scan(&c.ID, &c.Phone, &c.Email, &c.Name, &c.Discount, &c.FileID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}

	return contacts, nil
}

func (s *PostgresStorage) UpdateContact(ctx context.Context, contact models.Contact) error {
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(queryCtx, `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4, file_id = $5, updated_at = now()
		WHERE id = $6
	`, contact.Phone, contact.Email, contact.Name, contact.Discount, contact.FileID, contact.ID)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	s.saveContactVersion(ctx, contact, "updated")

	return nil
}

func (s *PostgresStorage) scanContact(ctx context.Context, whereClause string, args ...any) (models.Contact, error) {
	var c models.Contact

	err := s.pool.QueryRow(ctx, `
		SELECT id, phone, email, name, discount, file_id, created_at, updated_at
		FROM contacts
		`+whereClause, args...).Scan(
		&c.ID, &c.Phone, &c.Email, &c.Name, &c.Discount,
		&c.FileID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return models.Contact{}, err
	}

	return c, nil
}

func (s *PostgresStorage) ResolveConflict(ctx context.Context, phone string, action models.ConflictAction, incoming models.Contact) error {
	existing, _, err := s.GetContactByPhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("resolve conflict get existing: %w", err)
	}

	switch action {
	case models.ConflictActionSkip:
		return nil
	case models.ConflictActionReplace:
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		return s.UpdateContact(ctx, incoming)
	case models.ConflictActionMerge:
		if incoming.Name == "" {
			incoming.Name = existing.Name
		}
		if incoming.Email == "" {
			incoming.Email = existing.Email
		}
		if incoming.Discount == "" {
			incoming.Discount = existing.Discount
		}
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		return s.UpdateContact(ctx, incoming)
	default:
		return fmt.Errorf("unknown conflict action: %s", action)
	}
}

func (s *PostgresStorage) saveContactVersion(ctx context.Context, contact models.Contact, action string) error {
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(queryCtx, `
		INSERT INTO contact_versions (contact_id, phone, email, name, discount, file_id, action)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, contact.ID, contact.Phone, contact.Email, contact.Name, contact.Discount, contact.FileID, action)
	return err
}
