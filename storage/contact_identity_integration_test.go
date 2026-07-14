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

// TestCleanPostgresSchemaAndContactResolution проверяет схему и разрешение конфликтов.
func TestCleanPostgresSchemaAndContactResolution(t *testing.T) {
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
	if err := MigratePostgres(ctx, databaseURL); err != nil {
		t.Fatalf("MigratePostgres repeated run: %v", err)
	}
	if err := RollbackPostgresMigration(ctx, databaseURL); err != nil {
		t.Fatalf("RollbackPostgresMigration: %v", err)
	}
	if err := MigratePostgres(ctx, databaseURL); err != nil {
		t.Fatalf("MigratePostgres after rollback: %v", err)
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

	const legacySchemaQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND (
				(table_name = 'contacts' AND column_name IN ('data', 'file_id')) OR
				(table_name = 'uploaded_files' AND column_name = 'payload') OR
				(table_name = 'contact_sources' AND column_name = 'incoming')
			  )
		) OR EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
			  AND table_name = 'contact_versions'
		)
	`
	var legacySchemaExists bool
	if err := store.pool.QueryRow(ctx, legacySchemaQuery).Scan(&legacySchemaExists); err != nil {
		t.Fatalf("check legacy schema: %v", err)
	}
	if legacySchemaExists {
		t.Fatal("database must not contain legacy columns or contact_versions")
	}

	incoming := models.Contact{
		Phone:     phone,
		Name:      "Пётр",
		FileID:    fileID,
		SourceRow: 2,
	}
	if err := store.ResolveConflict(ctx, phone, models.ConflictActionSkip, incoming); err != nil {
		t.Fatalf("ResolveConflict skip: %v", err)
	}
	if err := store.ResolveConflict(ctx, phone, models.ConflictActionReplace, incoming); err != nil {
		t.Fatalf("ResolveConflict replace: %v", err)
	}
	if err := store.ResolveConflict(ctx, phone, models.ConflictActionReplace, incoming); err != nil {
		t.Fatalf("ResolveConflict repeated replace: %v", err)
	}

	contact, found, err = store.GetContactByPhone(ctx, phone)
	if err != nil || !found || contact.Name != incoming.Name {
		t.Fatalf("resolved contact: found=%v contact=%#v err=%v", found, contact, err)
	}

	const sourceStateQuery = `
		SELECT count(*), max(action)
		FROM contact_sources
		WHERE contact_id = $1 AND file_id = $2 AND row_number = $3
	`
	var sourceCount int
	var sourceAction string
	if err := store.pool.QueryRow(ctx, sourceStateQuery, contact.ID, fileID, 2).Scan(&sourceCount, &sourceAction); err != nil {
		t.Fatalf("read contact source: %v", err)
	}
	if sourceCount != 1 || sourceAction != string(models.ContactSourceReplaced) {
		t.Fatalf("contact source count=%d action=%q", sourceCount, sourceAction)
	}

	fixedPhone := fmt.Sprintf("+7 (998) %s-%s-%s", phoneDigits[0:3], phoneDigits[3:5], phoneDigits[5:7])
	fixedFileID, err := store.SaveFileData(ctx, models.FileData{
		OriginalFilename: "fixed-row.csv",
		Format:           "csv",
		Headers:          []string{"Телефон", "Имя"},
		Rows:             []map[string]string{{"Телефон": "bad", "Имя": "Анна"}},
		RowNumbers:       []int{2},
		Stats: models.ProcessingStats{
			RowCount:        1,
			ColumnCount:     2,
			InvalidRowCount: 1,
		},
		Warnings: []models.ProcessingWarning{{Row: 2, Column: "Телефон", Message: "Некорректный телефон."}},
		InvalidRows: []models.InvalidRow{{
			Row:    2,
			Values: map[string]string{"Телефон": "bad", "Имя": "Анна"},
			Errors: []models.ProcessingWarning{{Row: 2, Column: "Телефон", Message: "Некорректный телефон."}},
		}},
	})
	if err != nil {
		t.Fatalf("SaveFileData fixed row: %v", err)
	}

	fixedValues := map[string]string{"Телефон": fixedPhone, "Имя": "Анна"}
	if err := store.SaveFixedRow(ctx, fixedFileID, 2, fixedValues, models.Contact{
		Phone: fixedPhone,
		Name:  "Анна",
	}); err != nil {
		t.Fatalf("SaveFixedRow: %v", err)
	}

	refreshed, found, err := store.GetFileData(ctx, fixedFileID)
	if err != nil || !found {
		t.Fatalf("GetFileData fixed row: found=%v err=%v", found, err)
	}
	if len(refreshed.InvalidRows) != 0 || len(refreshed.Warnings) != 0 {
		t.Fatalf("fixed row still has validation errors: %#v", refreshed)
	}
	if refreshed.Stats.ValidRowCount != 1 || refreshed.Stats.InvalidRowCount != 0 {
		t.Fatalf("unexpected refreshed stats: %#v", refreshed.Stats)
	}
	if refreshed.Rows[0]["Телефон"] != fixedPhone {
		t.Fatalf("fixed row phone = %q", refreshed.Rows[0]["Телефон"])
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
