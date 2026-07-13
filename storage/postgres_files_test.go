package storage

import (
	"testing"

	"task1/models"
)

// TestEscapeLikePattern доказывает, что %, _ и \ ищутся как текст, а не как маски LIKE.
func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "plain", value: "+79", want: "+79"},
		{name: "percent", value: "50%", want: `50\%`},
		{name: "underscore", value: "user_name", want: `user\_name`},
		{name: "backslash", value: `a\b`, want: `a\\b`},
		{name: "combined", value: `50%_a\b`, want: `50\%\_a\\b`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := escapeLikePattern(test.value); got != test.want {
				t.Fatalf("escapeLikePattern(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

// TestSourceRowNumber проверяет приорит RowNumbers и резервный расчёт от HeaderRow.
func TestSourceRowNumber(t *testing.T) {
	tests := []struct {
		name  string
		data  models.FileData
		index int
		want  int
	}{
		{name: "stored source row", data: models.FileData{HeaderRow: 1, RowNumbers: []int{2, 7}}, index: 1, want: 7},
		{name: "after header", data: models.FileData{HeaderRow: 3}, index: 1, want: 5},
		{name: "without header", index: 1, want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sourceRowNumber(test.data, test.index); got != test.want {
				t.Fatalf("sourceRowNumber() = %d, want %d", got, test.want)
			}
		})
	}
}
