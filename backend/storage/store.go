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

func NewFromEnv(ctx context.Context) (FileStore, error) {
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
