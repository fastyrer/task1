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
	Ping(ctx context.Context) error
	Close()
	Driver() string
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

	Close()
	Driver() string
}

// CombinedStore – объединение интерфейсов
type CombinedStore interface {
	FileStore
	ContactStore
}

// NewFromEnv – создает хранилище исходя из драйвера в .env. Реализует CombinedStorage
func NewFromEnv(ctx context.Context) (CombinedStore, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_DRIVER")))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))

	// Если драйвер не задан в окружении
	if driver == "" {
		if databaseURL != "" {
			driver = "postgres"
		} else {
			driver = "memory"
		}
	}

	// По драйверу создается нужная реализация
	switch driver {
	case "memory", "in-memory":
		return NewMemoryStorage(), nil
	case "postgres", "postgresql":
		if databaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required when STORAGE_DRIVER=%s", driver)
		}
		return NewPostgresStorage(ctx, databaseURL)
	default:
		return nil, fmt.Errorf("unsupported STORAGE_DRIVER %q", driver)
	}
}
