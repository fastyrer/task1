package storage

import "testing"

func TestContactSearchPatternEscapesLikeMetacharacters(t *testing.T) {
	got := contactSearchPattern(`50%_\test`)
	want := `%50\%\_\\test%`
	if got != want {
		t.Fatalf("contactSearchPattern() = %q, want %q", got, want)
	}
}
