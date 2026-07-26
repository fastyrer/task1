package storage

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"task1/internal/models"
)

// TestPostgresIntegration проверяет миграции, триггер индексируемого поиска и
// потоковый обход контактов на реальном PostgreSQL. По умолчанию тест
// пропускается; CI передаёт отдельную TEST_DATABASE_URL.
func TestPostgresIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	t.Cleanup(adminPool.Close)

	var databaseName string
	if err := adminPool.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read integration database name: %v", err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Fatalf("refusing to reset schema in non-test database %q", databaseName)
	}

	const schema = "task1_integration"
	if _, err := adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		t.Fatalf("drop old integration schema: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create integration schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = adminPool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})

	schemaURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse integration database URL: %v", err)
	}
	query := schemaURL.Query()
	query.Set("search_path", schema+",public")
	schemaURL.RawQuery = query.Encode()

	if err := MigratePostgres(ctx, schemaURL.String()); err != nil {
		t.Fatalf("apply integration migrations: %v", err)
	}
	store, err := NewPostgresStorage(ctx, schemaURL.String())
	if err != nil {
		t.Fatalf("open migrated integration storage: %v", err)
	}
	defer store.Close()

	_, err = store.pool.Exec(ctx, `
		INSERT INTO contacts (phone, email, name, discount)
		VALUES
			('+79991234567', 'percent@example.test', 'Literal % User', '10'),
			('+79997654321', 'regular@example.test', 'Regular User', '20')
	`)
	if err != nil {
		t.Fatalf("insert integration contacts: %v", err)
	}

	contacts, total, err := store.ListContactsPage(ctx, "%", 25, 0)
	if err != nil {
		t.Fatalf("search literal wildcard: %v", err)
	}
	if total != 1 || len(contacts) != 1 || contacts[0].Name != "Literal % User" {
		t.Fatalf("literal wildcard search returned total=%d contacts=%+v", total, contacts)
	}

	visited := 0
	if err := store.VisitContacts(ctx, func(_ models.Contact) error {
		visited++
		return nil
	}); err != nil {
		t.Fatalf("visit integration contacts: %v", err)
	}
	if visited != 2 {
		t.Fatalf("visited %d contacts, want 2", visited)
	}

	var indexExists bool
	if err := store.pool.QueryRow(
		ctx,
		"SELECT to_regclass($1) IS NOT NULL",
		schema+".contacts_search_text_trgm_idx",
	).Scan(&indexExists); err != nil {
		t.Fatalf("check contact search index: %v", err)
	}
	if !indexExists {
		t.Fatal("contacts search trigram index was not created")
	}
}
