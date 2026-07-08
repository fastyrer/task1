package services

import (
	"testing"
)

func TestParsePlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{"простой шаблон", "{{Имя}}", []string{"Имя"}},
		{"несколько плейсхолдеров", "{{Имя}} {{Скидка}}", []string{"Имя", "Скидка"}},
		{"без плейсхолдеров", "просто текст", nil},
		{"пустая строка", "", nil},
		{"пробелы внутри", "{{ Имя }}", []string{"Имя"}},
		{"дубликаты", "{{Имя}} {{Имя}}", []string{"Имя"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePlaceholders(tt.template)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestValidateUnknownPlaceholders(t *testing.T) {
	headers := []string{"Имя", "Телефон", "Скидка"}

	t.Run("все плейсхолдеры известны", func(t *testing.T) {
		err := ValidateUnknownPlaceholders([]string{"Имя", "Скидка"}, headers)
		if err != nil {
			t.Fatal("ожидался nil, получил:", err)
		}
	})

	t.Run("неизвестный плейсхолдер", func(t *testing.T) {
		err := ValidateUnknownPlaceholders([]string{"Имя", "Адрес"}, headers)
		if err == nil {
			t.Fatal("ожидалась ошибка")
		}
	})
}

func TestGenerateText(t *testing.T) {
	row := map[string]string{
		"Имя":    "Анна",
		"Скидка": "15",
	}

	t.Run("подстановка значений", func(t *testing.T) {
		result := GenerateText("Привет, {{Имя}}! Скидка: {{Скидка}}%", row)
		want := "Привет, Анна! Скидка: 15%"
		if result != want {
			t.Fatalf("got %q, want %q", result, want)
		}
	})

	t.Run("нет плейсхолдеров", func(t *testing.T) {
		result := GenerateText("просто текст", row)
		if result != "просто текст" {
			t.Fatalf("got %q, want %q", result, "просто текст")
		}
	})

	t.Run("неизвестный плейсхолдер игнорируется", func(t *testing.T) {
		result := GenerateText("{{Неизвестный}}", row)
		if result != "" {
			t.Fatalf("got %q, want %q", result, "")
		}
	})
}
