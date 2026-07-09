package utils

import (
	"reflect"
	"testing"
)

func TestCleanHeader(t *testing.T) {
	got := CleanHeader("\ufeff  Имя\u00a0 клиента  ")
	if got != "Имя клиента" {
		t.Fatalf("unexpected header: %q", got)
	}
}

func TestTrimTrailingEmptyCells(t *testing.T) {
	got := TrimTrailingEmptyCells([]string{"Анна", " Москва ", "\u00a0", ""})
	want := []string{"Анна", " Москва "}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected cells: %#v", got)
	}
}

func TestRecordToMap(t *testing.T) {
	got := RecordToMap(
		[]string{"Имя", "Город", "Телефон"},
		[]string{" Анна ", "Москва"},
	)
	want := map[string]string{
		"Имя":     "Анна",
		"Город":   "Москва",
		"Телефон": "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected row: %#v", got)
	}
}

func TestCloneRowCreatesIndependentMap(t *testing.T) {
	original := map[string]string{"Имя": "Анна"}
	clone := CloneRow(original)
	clone["Имя"] = "Иван"

	if original["Имя"] != "Анна" {
		t.Fatalf("original row was modified: %#v", original)
	}
}

func TestRecordHelpers(t *testing.T) {
	if !IsEmptyRecord([]string{" ", "\u00a0"}) {
		t.Fatal("expected record to be empty")
	}
	if IsEmptyRecord([]string{"", "Анна"}) {
		t.Fatal("expected record to contain data")
	}
	if !ContainsString([]string{"CSV", "XLSX"}, "XLSX") {
		t.Fatal("expected value to be found")
	}
}
