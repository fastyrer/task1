package utils

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{name: "national", value: "9991234567", want: "+7 (999) 123-45-67", ok: true},
		{name: "leading eight", value: "8 (999) 123-45-67", want: "+7 (999) 123-45-67", ok: true},
		{name: "invalid country", value: "+420776654321", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizePhone(test.value)
			if ok != test.ok || got != test.want {
				t.Fatalf("NormalizePhone(%q) = %q, %v; want %q, %v", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	got, ok := NormalizeEmail(" USER@Example.COM ")
	if !ok || got != "user@example.com" {
		t.Fatalf("unexpected normalized email: %q, %v", got, ok)
	}

	if _, ok := NormalizeEmail("not an email"); ok {
		t.Fatal("expected invalid email")
	}
}

func TestNormalizePercent(t *testing.T) {
	got, ok := NormalizePercent("15,5%")
	if !ok || got != "15.5" {
		t.Fatalf("unexpected normalized percent: %q, %v", got, ok)
	}

	if _, ok := NormalizePercent("101"); ok {
		t.Fatal("expected percent above 100 to be invalid")
	}
}

func TestValueRecognition(t *testing.T) {
	if !IsSupportedDate("12.07.2026") {
		t.Fatal("expected date to be supported")
	}
	if IsSupportedDate("2026-99-99") {
		t.Fatal("expected date to be invalid")
	}
	if !IsNumberLike("15,5%") {
		t.Fatal("expected percentage to be number-like")
	}
}
