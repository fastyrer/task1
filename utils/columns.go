package utils

import "strings"

type ColumnKind string

const (
	ColumnGeneric  ColumnKind = ""
	ColumnPhone    ColumnKind = "phone"
	ColumnEmail    ColumnKind = "email"
	ColumnDiscount ColumnKind = "discount"
	ColumnDate     ColumnKind = "date"
)

// ClassifyHeader определяет назначение колонки по её заголовку.
func ClassifyHeader(header string) ColumnKind {
	key := HeaderKey(header)
	switch {
	case strings.Contains(key, "телефон"), key == "phone", strings.Contains(key, "mobile"):
		return ColumnPhone
	case key == "email", key == "e-mail", strings.Contains(key, "почта"), strings.Contains(key, "mail"):
		return ColumnEmail
	case strings.Contains(key, "скидка"), strings.Contains(key, "discount"), strings.Contains(key, "процент"):
		return ColumnDiscount
	case strings.Contains(key, "дата"), key == "date", strings.HasSuffix(key, " date"):
		return ColumnDate
	default:
		return ColumnGeneric
	}
}

// IsCommonHeader сообщает, похож ли текст на распространённый заголовок.
func IsCommonHeader(header string) bool {
	key := HeaderKey(header)
	if ClassifyHeader(header) != ColumnGeneric {
		return true
	}

	switch key {
	case "имя", "фио", "name", "first name", "last name", "клиент", "client", "город", "city":
		return true
	default:
		return false
	}
}

// HeaderKey приводит заголовок к форме, пригодной для сравнения.
func HeaderKey(header string) string {
	key := strings.ToLower(CleanHeader(header))
	return strings.ReplaceAll(key, "ё", "е")
}

// DetectPhoneColumn возвращает первый заголовок телефонной колонки.
func DetectPhoneColumn(headers []string) string {
	for _, header := range headers {
		if ClassifyHeader(header) == ColumnPhone {
			return header
		}
	}

	return ""
}
