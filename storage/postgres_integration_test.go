package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"task1/models"
)

// TestPostgresStorageRoundTrip проверяет полный сценарий на реальном PostgreSQL:
// миграцию, файл и строки, поиск, UNIQUE-телефон, replace и аудит.
func TestPostgresStorageRoundTrip(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	assertTestDatabase(t, databaseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := MigratePostgres(ctx, databaseURL); err != nil {
		t.Fatalf("MigratePostgres: %v", err)
	}
	store, err := NewPostgresStorage(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresStorage: %v", err)
	}
	defer store.Close()

	phoneDigits := fmt.Sprintf("%07d", time.Now().UnixNano()%10000000)
	phone := fmt.Sprintf("+7 (999) %s-%s-%s", phoneDigits[0:3], phoneDigits[3:5], phoneDigits[5:7])

	file := models.FileData{
		OriginalFilename: "integration.csv",
		Size:             128,
		MIMEType:         "text/csv",
		DetectedMIMEType: "text/csv",
		Format:           "csv",
		Encoding:         "UTF-8",
		HeaderRow:        1,
		Headers:          []string{"Телефон", "Имя", "Комментарий"},
		Rows: []map[string]string{
			{"Телефон": phone, "Имя": "Анна", "Комментарий": "Скидка 50%"},
			{"Телефон": "broken", "Имя": "Иван", "Комментарий": "Ошибка"},
		},
		RowNumbers: []int{2, 4},
		Stats: models.ProcessingStats{
			RowCount:        2,
			ColumnCount:     3,
			ValidRowCount:   1,
			InvalidRowCount: 1,
			WarningCount:    1,
		},
		Warnings: []models.ProcessingWarning{
			{Row: 4, Column: "Телефон", Message: "Некорректный телефон."},
		},
		InvalidRows: []models.InvalidRow{
			{
				Row:    4,
				Values: map[string]string{"Телефон": "broken", "Имя": "Иван", "Комментарий": "Ошибка"},
				Errors: []models.ProcessingWarning{
					{Row: 4, Column: "Телефон", Message: "Некорректный телефон."},
				},
			},
		},
	}

	fileID, err := store.SaveFileData(ctx, file)
	if err != nil {
		t.Fatalf("SaveFileData: %v", err)
	}

	loaded, found, err := store.GetFileData(ctx, fileID)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !found {
		t.Fatal("GetFileData: file not found")
	}
	if len(loaded.RowNumbers) != 2 || loaded.RowNumbers[0] != 2 || loaded.RowNumbers[1] != 4 {
		t.Fatalf("row numbers were not preserved: %#v", loaded.RowNumbers)
	}
	if len(loaded.InvalidRows) != 1 || loaded.InvalidRows[0].Row != 4 {
		t.Fatalf("invalid rows were not preserved: %#v", loaded.InvalidRows)
	}

	search, found, err := store.SearchFileRows(ctx, fileID, "50%", 10)
	if err != nil {
		t.Fatalf("SearchFileRows: %v", err)
	}
	if !found || search.Total != 1 || len(search.Rows) != 1 || search.Rows[0].Row != 2 {
		t.Fatalf("unexpected search result: %#v", search)
	}

	contact := models.Contact{
		Phone:     phone,
		Name:      "Анна",
		Email:     "anna@example.com",
		Discount:  "50",
		Data:      map[string]string{"Комментарий": "Скидка 50%"},
		FileID:    fileID,
		SourceRow: 2,
	}
	if _, err := store.SaveContact(ctx, contact); err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	if _, err := store.SaveContact(ctx, contact); !errors.Is(err, ErrContactAlreadyExists) {
		t.Fatalf("duplicate SaveContact error = %v, want ErrContactAlreadyExists", err)
	}

	incoming := contact
	incoming.Name = "Анна Сергеевна"
	if err := store.ResolveConflict(ctx, contact.Phone, models.ConflictActionReplace, incoming); err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}

	updated, found, err := store.GetContactByPhone(ctx, contact.Phone)
	if err != nil {
		t.Fatalf("GetContactByPhone: %v", err)
	}
	if !found || updated.Name != incoming.Name {
		t.Fatalf("contact was not replaced: %#v", updated)
	}

	const auditQuery = `
		SELECT count(*)
		FROM contact_versions
		WHERE contact_id = $1
		  AND action IN ('created', 'replaced')
	`
	var auditCount int
	if err := store.pool.QueryRow(ctx, auditQuery, updated.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count contact versions: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("contact version count = %d, want 2", auditCount)
	}
}

// assertTestDatabase не даёт интеграционному тесту случайно подключиться к production-базе.
// Имя тестовой БД обязано содержать "test".
func assertTestDatabase(t *testing.T, databaseURL string) {
	t.Helper()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if !strings.Contains(strings.ToLower(config.ConnConfig.Database), "test") {
		t.Fatalf("TEST_DATABASE_URL must point to a database whose name contains 'test', got %q", config.ConnConfig.Database)
	}
}
