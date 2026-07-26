package services

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRequiredContactColumnsAcceptsAliases(t *testing.T) {
	err := validateRequiredContactColumns([]string{"phone", "name", "e-mail", "discount"})
	if err != nil {
		t.Fatalf("validateRequiredContactColumns() error = %v", err)
	}
}

func TestValidateRequiredContactColumnsRejectsEachMissingKind(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		missing string
	}{
		{name: "phone", headers: []string{"ФИО", "Email", "Скидка"}, missing: "«Телефон»"},
		{name: "name", headers: []string{"Телефон", "Email", "Скидка"}, missing: "«ФИО»"},
		{name: "email", headers: []string{"Телефон", "ФИО", "Скидка"}, missing: "«Email»"},
		{name: "discount", headers: []string{"Телефон", "ФИО", "Email"}, missing: "«Скидка»"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequiredContactColumns(test.headers)
			if !errors.Is(err, ErrMissingRequiredColumns) || !strings.Contains(err.Error(), test.missing) {
				t.Fatalf("validateRequiredContactColumns() error = %v, want missing %s", err, test.missing)
			}
		})
	}
}

func TestRowsToFileDataRejectsMissingRequiredColumns(t *testing.T) {
	_, err := rowsToFileData([][]string{
		{"Телефон", "ФИО"},
		{"+79991234567", "Иван Иванов"},
	})
	if !errors.Is(err, ErrMissingRequiredColumns) {
		t.Fatalf("rowsToFileData() error = %v, want %v", err, ErrMissingRequiredColumns)
	}
	if !strings.Contains(err.Error(), "«Email», «Скидка»") {
		t.Fatalf("missing columns error = %q", err)
	}
}

func TestRowsToFileDataAllowsEmptyOptionalValues(t *testing.T) {
	data, err := rowsToFileData([][]string{
		{"Телефон", "ФИО", "Email", "Скидка"},
		{"+79991234567", "Иван Иванов", "", ""},
	})
	if err != nil {
		t.Fatalf("rowsToFileData() error = %v", err)
	}
	if data.Stats.ValidRowCount != 1 || data.Stats.InvalidRowCount != 0 {
		t.Fatalf("unexpected stats: %+v", data.Stats)
	}
}

func TestRowsToFileDataMarksEmptyRequiredValue(t *testing.T) {
	data, err := rowsToFileData([][]string{
		{"Телефон", "ФИО", "Email", "Скидка"},
		{"+79991234567", "", "", ""},
	})
	if err != nil {
		t.Fatalf("rowsToFileData() error = %v", err)
	}
	if data.Stats.ValidRowCount != 0 || data.Stats.InvalidRowCount != 1 {
		t.Fatalf("unexpected stats: %+v", data.Stats)
	}
	if got := data.InvalidRows[0].Errors[0]; got.Column != "ФИО" || got.Message != "Обязательное поле не заполнено." {
		t.Fatalf("unexpected field error: %+v", got)
	}

	result := NewImportValidation(data, testImportID)
	if result.TableError == "" {
		t.Fatal("expected table-level validation error")
	}
}
