package storage

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"task1/models"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestContactSerialIDAndGeneratedUID проверяет конечную схему на отдельной тестовой БД.
func TestContactSerialIDAndGeneratedUID(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	assertContactTestDatabase(t, databaseURL)

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

	fileID, err := store.SaveFileData(ctx, models.FileData{
		OriginalFilename: "contact-identity.csv",
		Format:           "csv",
		Headers:          []string{"Телефон", "Имя"},
		Rows:             []map[string]string{{"Телефон": phone, "Имя": "Анна"}},
		RowNumbers:       []int{2},
	})
	if err != nil {
		t.Fatalf("SaveFileData: %v", err)
	}

	uid, err := store.SaveContact(ctx, models.Contact{
		Phone:     phone,
		Name:      "Анна",
		FileID:    fileID,
		SourceRow: 2,
	})
	if err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	if !uuidPattern.MatchString(uid) {
		t.Fatalf("SaveContact UID = %q, want UUID", uid)
	}

	contact, found, err := store.GetContactByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("GetContactByPhone: %v", err)
	}
	if !found || contact.ID <= 0 || contact.UID != uid {
		t.Fatalf("unexpected contact identity: %#v", contact)
	}

	const dataColumnQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name IN ('contacts', 'contact_versions')
			  AND column_name = 'data'
		)
	`
	var dataColumnExists bool
	if err := store.pool.QueryRow(ctx, dataColumnQuery).Scan(&dataColumnExists); err != nil {
		t.Fatalf("check data columns: %v", err)
	}
	if dataColumnExists {
		t.Fatal("contacts schema must not contain data columns")
	}
}

// assertContactTestDatabase запрещает запуск интеграционного теста на рабочей БД.
func assertContactTestDatabase(t *testing.T, databaseURL string) {
	t.Helper()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if !strings.Contains(strings.ToLower(config.ConnConfig.Database), "test") {
		t.Fatalf("TEST_DATABASE_URL must point to a database whose name contains test, got %q", config.ConnConfig.Database)
	}
}
