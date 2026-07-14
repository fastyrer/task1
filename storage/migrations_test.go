package storage

import "testing"

func TestLoadEmbeddedMigrations(t *testing.T) {
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatalf("loadEmbeddedMigrations: %v", err)
	}
	if len(migrations) != 1 {
		t.Fatalf("migration count = %d, want 1", len(migrations))
	}

	migration := migrations[0]
	if migration.Version != 1 {
		t.Fatalf("migration version = %d, want 1", migration.Version)
	}
	if migration.Name != "000001_initial_schema.up.sql" {
		t.Fatalf("migration name = %q", migration.Name)
	}
	if len(migration.Script) == 0 {
		t.Fatal("migration script is empty")
	}
	if len(migration.Checksum) != 64 {
		t.Fatalf("migration checksum length = %d, want 64", len(migration.Checksum))
	}
}

func TestMigrationVersionRejectsInvalidNames(t *testing.T) {
	invalidNames := []string{
		"initial_schema.up.sql",
		"000000_initial_schema.up.sql",
		"invalid_initial_schema.up.sql",
	}
	for _, name := range invalidNames {
		if _, err := migrationVersion(name); err == nil {
			t.Fatalf("migrationVersion(%q) returned no error", name)
		}
	}
}
