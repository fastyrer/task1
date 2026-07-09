package utils

import "testing"

func TestClassifyHeader(t *testing.T) {
	tests := map[string]ColumnKind{
		"Телефон":       ColumnPhone,
		"E-mail":        ColumnEmail,
		"Скидка":        ColumnDiscount,
		"Дата доставки": ColumnDate,
		"Комментарий":   ColumnGeneric,
	}

	for header, want := range tests {
		if got := ClassifyHeader(header); got != want {
			t.Fatalf("ClassifyHeader(%q) = %q; want %q", header, got, want)
		}
	}
}

func TestHeaderHelpers(t *testing.T) {
	if got := HeaderKey("  ТЕЛЕФОН\u00a0 "); got != "телефон" {
		t.Fatalf("unexpected header key: %q", got)
	}
	if got := HeaderKey("ВСЁ"); got != "все" {
		t.Fatalf("unexpected normalized ё: %q", got)
	}
	if !IsCommonHeader("Город") {
		t.Fatal("expected common header")
	}
	if got := DetectPhoneColumn([]string{"Имя", "Мобильный телефон"}); got != "Мобильный телефон" {
		t.Fatalf("unexpected phone column: %q", got)
	}
}
