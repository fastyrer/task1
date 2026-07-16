// Package storage определяет минимальные контракты PostgreSQL.
package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"task1/models"
)

// ContactReader используется рассылкой и не даёт ей методов изменения контактов.
type ContactReader interface {
	ListContacts(ctx context.Context) ([]models.Contact, error)
}

// ContactPageReader читает одну страницу справочника с необязательным поиском.
type ContactPageReader interface {
	ListContactsPage(ctx context.Context, query string, limit, offset int) ([]models.Contact, int64, error)
}

// ContactUpdater изменяет только одну актуальную запись с проверкой её версии.
type ContactUpdater interface {
	UpdateContact(ctx context.Context, uid string, expectedUpdatedAt time.Time, contact models.Contact) (models.Contact, error)
}

// ContactDirectoryStore объединяет операции, разрешённые вкладке «Контакты».
type ContactDirectoryStore interface {
	ContactPageReader
	ContactUpdater
}

// ImportStore разделяет read-only предпросмотр и атомарный подтверждённый импорт.
type ImportStore interface {
	// FindContactsByPhones читает только контакты, необходимые для предпросмотра.
	FindContactsByPhones(ctx context.Context, phones []string) (map[string]models.Contact, error)
	// CommitImport является единственной операцией, записывающей импорт в PostgreSQL.
	CommitImport(
		ctx context.Context,
		data models.FileData,
		contacts []models.Contact,
		decisions map[string]models.ImportDecision,
	) (models.ImportCommitResult, error)
}

// HealthStore - минимальный контракт проверки доступности БД для /api/health.
type HealthStore interface {
	Ping(ctx context.Context) error
}

// Store - полный контракт хранилища, который нужен main.go.
// Единственная runtime-реализация этого интерфейса - PostgresStorage.
type Store interface {
	ImportStore
	ContactReader
	ContactDirectoryStore
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

// RollbackMigrationFromEnv - откатывает последнюю миграцию из DATABASE_URL.
func RollbackMigrationFromEnv(ctx context.Context) error {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return err
	}
	return RollbackPostgresMigration(ctx, databaseURL)
}

// databaseURLFromEnv - читает и проверяет обязательную строку подключения.
func databaseURLFromEnv() (string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return "", fmt.Errorf("DATABASE_URL is required")
	}
	return databaseURL, nil
}
