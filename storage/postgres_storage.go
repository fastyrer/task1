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

	"task1/models"
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
	data JSONB NOT NULL DEFAULT '{}',
	file_id TEXT NOT NULL REFERENCES uploaded_files(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS 	-- phone уже UNIQUE, индекс создаётся автоматически
CREATE INDEX IF NOT EXISTS contacts_file_id_idx ON contacts (file_id);
CREATE INDEX IF NOT EXISTS contacts_email_idx ON contacts (email);

CREATE TABLE IF NOT EXISTS contact_versions (
	id SERIAL PRIMARY KEY,
	contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
	phone TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	discount TEXT NOT NULL DEFAULT '',
	data JSONB NOT NULL DEFAULT '{}',
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

func (s *PostgresStorage) SaveContact(ctx context.Context, contact models.Contact) (string, error) {
	contactID := strings.TrimSpace(contact.ID)
	if contactID == "" {
		contactID = generateFileID()
	}
	contact.ID = contactID

	dataJSON, err := marshalContactJSON(&contact)
	if err != nil {
		return "", err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err = s.pool.Exec(queryCtx, `
		INSERT INTO contacts (id, phone, email, name, discount, data, file_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			phone = EXCLUDED.phone,
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			discount = EXCLUDED.discount,
			data = EXCLUDED.data,
			file_id = EXCLUDED.file_id,
			updated_at = now()
	`, contactID, contact.Phone, contact.Email, contact.Name, contact.Discount, dataJSON, contact.FileID)
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
		SELECT id, phone, email, name, discount, data, file_id, created_at, updated_at
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
		var dataJSON []byte
		if err := rows.Scan(&c.ID, &c.Phone, &c.Email, &c.Name, &c.Discount, &dataJSON, &c.FileID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		if dataJSON != nil {
			json.Unmarshal(dataJSON, &c.Data)
		}
		contacts = append(contacts, c)
	}

	return contacts, nil
}

func (s *PostgresStorage) UpdateContact(ctx context.Context, contact models.Contact) error {
	dataJSON, err := marshalContactJSON(&contact)
	if err != nil {
		return err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err = s.pool.Exec(queryCtx, `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4, data = $5, file_id = $6, updated_at = now()
		WHERE id = $7
	`, contact.Phone, contact.Email, contact.Name, contact.Discount, dataJSON, contact.FileID, contact.ID)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	s.saveContactVersion(ctx, contact, "updated")

	return nil
}

// scanContact – вспомогательный метод, читает одну строку из contacts по условию WHERE.
// Принимает контекст, условие (например "WHERE phone = $1") и параметры.
// Возвращает заполненный Contact или pgx.ErrNoRows, если строка не найдена.
func (s *PostgresStorage) scanContact(ctx context.Context, whereClause string, args ...any) (models.Contact, error) {
	var c models.Contact
	var dataJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, phone, email, name, discount, data, file_id, created_at, updated_at
		FROM contacts
		`+whereClause, args...).Scan(
		&c.ID, &c.Phone, &c.Email, &c.Name, &c.Discount,
		&dataJSON, &c.FileID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return models.Contact{}, err
	}

	if dataJSON != nil {
		json.Unmarshal(dataJSON, &c.Data)
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
		if incoming.Data == nil {
			incoming.Data = existing.Data
		} else {
			for k, v := range existing.Data {
				if _, ok := incoming.Data[k]; !ok {
					incoming.Data[k] = v
				}
			}
		}
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		return s.UpdateContact(ctx, incoming)
	default:
		return fmt.Errorf("unknown conflict action: %s", action)
	}
}

func (s *PostgresStorage) saveContactVersion(ctx context.Context, contact models.Contact, action string) error {
	dataJSON, err := marshalContactJSON(&contact)
	if err != nil {
		return err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err = s.pool.Exec(queryCtx, `
		INSERT INTO contact_versions (contact_id, phone, email, name, discount, data, file_id, action)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, contact.ID, contact.Phone, contact.Email, contact.Name, contact.Discount, dataJSON, contact.FileID, action)
	return err
}

// marshalContactJSON – marshals contact.Data into JSON bytes.
// Паникует, если contact nil (программная ошибка, не рантайм).
func marshalContactJSON(contact *models.Contact) ([]byte, error) {
	dataJSON, err := json.Marshal(contact.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal contact data: %w", err)
	}
	return dataJSON, nil
}
