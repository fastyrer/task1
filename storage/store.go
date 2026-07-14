// Package storage определяет интерфейсы и PostgreSQL-реализацию
// хранилища файлов, строк и контактов.
package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"task1/models"
)

// FileStore - операции с метаданными файла, его строками и поиском.
type FileStore interface {
	// SaveFileData сохраняет файл и все его строки одной транзакцией.
	SaveFileData(ctx context.Context, data models.FileData) (string, error)
	// GetFileData восстанавливает FileData из метаданных и таблицы строк.
	GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error)
	// SearchFileRows ищет подстроку внутри строк средствами PostgreSQL.
	SearchFileRows(ctx context.Context, fileID, query string, limit int) (models.FileSearchResult, bool, error)
}

// ContactStore – операции с контактами
type ContactStore interface {

	// SaveContact создаёт контакт и возвращает его публичный UID.
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

// HealthStore - минимальный контракт проверки доступности БД для /api/health.
type HealthStore interface {
	Ping(ctx context.Context) error
}

// Store - полный контракт хранилища, который нужен main.go.
// Единственная runtime-реализация этого интерфейса - PostgresStorage.
type Store interface {
	FileStore
	ContactStore
	HealthStore
	Close()
}

// NewFromEnv - создаёт PostgreSQL-хранилище из DATABASE_URL.
// In-memory fallback нет: ошибка конфигурации должна остановить запуск.
func NewFromEnv(ctx context.Context) (Store, error) {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return nil, err
	}
	return NewPostgresStorage(ctx, databaseURL)
}

// MigrateFromEnv - запускает версионированные миграции из DATABASE_URL.
// Эта функция вызывается отдельным режимом "server migrate".
func MigrateFromEnv(ctx context.Context) error {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return err
	}
	return MigratePostgres(ctx, databaseURL)
}

// databaseURLFromEnv - читает и проверяет обязательную строку подключения.
func databaseURLFromEnv() (string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return "", fmt.Errorf("DATABASE_URL is required")
	}
	return databaseURL, nil
}
