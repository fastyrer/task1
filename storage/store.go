package storage

// Package storage определяет интерфейсы для работы с хранилищами данных

import (
	"context"
	"fmt"
	"os"
	"strings"

	"task1/models"
)

// FileStore - операции с файлами
type FileStore interface {
	SaveFileData(ctx context.Context, data models.FileData) (string, error)
	GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error)
	SearchFileRows(ctx context.Context, fileID, query string, limit int) (models.FileSearchResult, bool, error)
}

// ContactStore – операции с контактами
type ContactStore interface {

	// SaveContact – создание и сохранение нового контакта
	SaveContact(ctx context.Context, contact models.Contact) (string, error)

	// GetContactByPhone – поиск контакта по телефону (телефон является уникальным ключом)
	GetContactByPhone(ctx context.Context, phone string) (models.Contact, bool, error)

	// ListContactsByFileID – все контакты из одного файла
	ListContactsByFileID(ctx context.Context, fileID string) ([]models.Contact, error)

	// UpdateContact – обновление уже существующего контакта
	UpdateContact(ctx context.Context, contact models.Contact) error

	// ResolveConflict – применить действие к конфликту
	ResolveConflict(ctx context.Context, phone string, action models.ConflictAction, incoming models.Contact) error
}

// HealthStore contains the database readiness operation used by /api/health.
type HealthStore interface {
	Ping(ctx context.Context) error
}

// Store is the complete PostgreSQL-backed application storage contract.
type Store interface {
	FileStore
	ContactStore
	HealthStore
	Close()
}

// NewFromEnv creates the only supported storage implementation: PostgreSQL.
func NewFromEnv(ctx context.Context) (Store, error) {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return nil, err
	}
	return NewPostgresStorage(ctx, databaseURL)
}

// MigrateFromEnv runs versioned PostgreSQL migrations using DATABASE_URL.
func MigrateFromEnv(ctx context.Context) error {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return err
	}
	return MigratePostgres(ctx, databaseURL)
}

func databaseURLFromEnv() (string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return "", fmt.Errorf("DATABASE_URL is required")
	}
	return databaseURL, nil
}
