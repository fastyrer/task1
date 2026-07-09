package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"task1/backend/models"
)

type FileStore interface {
	SaveFileData(ctx context.Context, data models.FileData) (string, error)
	GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error)
	Ping(ctx context.Context) error
	Close()
	Driver() string
}

type ContactStore interface {
	SaveContact(ctx context.Context, contact models.Contact) (string, error)
	GetContactByPhone(ctx context.Context, phone string) (models.Contact, bool, error)
	ListContactsByFileID(ctx context.Context, fileID string) ([]models.Contact, error)
	UpdateContact(ctx context.Context, contact models.Contact) error
	ResolveConflict(ctx context.Context, phone string, action models.ConflictAction, incoming models.Contact) error
	Close()
	Driver() string
}

type CombinedStore interface {
	FileStore
	ContactStore
}

func NewFromEnv(ctx context.Context) (CombinedStore, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_DRIVER")))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))

	if driver == "" {
		if databaseURL != "" {
			driver = "postgres"
		} else {
			driver = "memory"
		}
	}

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
