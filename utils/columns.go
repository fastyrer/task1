// columns.go – классификация колонок по заголовкам.

package utils

import "strings"

// ColumnKind – тип колонки.
type ColumnKind string

const (
	ColumnGeneric  ColumnKind = ""
	ColumnPhone    ColumnKind = "phone"
	ColumnName     ColumnKind = "name"
	ColumnEmail    ColumnKind = "email"
	ColumnDiscount ColumnKind = "discount"
)

// ClassifyHeader определяет тип колонки по заголовку.
func ClassifyHeader(header string) ColumnKind {
	key := strings.ToLower(header)
	switch {
	case strings.Contains(key, "телефон"), key == "phone", strings.Contains(key, "mobile"):
		return ColumnPhone
	case strings.Contains(key, "имя"), key == "name", strings.Contains(key, "фио"), strings.Contains(key, "клиент"), strings.Contains(key, "client"):
		return ColumnName
	case key == "email", key == "e-mail", strings.Contains(key, "почта"), strings.Contains(key, "mail"):
		return ColumnEmail
	case strings.Contains(key, "скидка"), strings.Contains(key, "discount"), strings.Contains(key, "процент"):
		return ColumnDiscount
	default:
		return ColumnGeneric
	}
}

// DetectPhoneColumn возвращает первый заголовок, распознанный как телефон.
func DetectPhoneColumn(headers []string) string {
	for _, header := range headers {
		if ClassifyHeader(header) == ColumnPhone {
			return header
		}
	}

	return ""
}
