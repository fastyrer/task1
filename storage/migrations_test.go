package storage

import "testing"

func TestMigrationVersion(t *testing.T) {
	tests := []struct {
		name    string
		want    int64
		wantErr bool
	}{
		{name: "000001_initial.up.sql", want: 1},
		{name: "000125_add_index.up.sql", want: 125},
		{name: "invalid.sql", wantErr: true},
		{name: "000000_invalid.up.sql", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := migrationVersion(test.name)
			if test.wantErr {
				if err == nil {
					t.Fatalf("migrationVersion(%q) expected error", test.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationVersion(%q): %v", test.name, err)
			}
			if got != test.want {
				t.Fatalf("migrationVersion(%q) = %d, want %d", test.name, got, test.want)
			}
		})
	}
}
