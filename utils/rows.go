// Package utils содержит небольшие переиспользуемые функции обработки данных.
package utils

import "strings"

// CleanHeader нормализует пробелы и удаляет BOM из заголовка.
func CleanHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}

// CleanCell заменяет неразрывные пробелы и обрезает края значения.
func CleanCell(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.TrimSpace(value)
}

// TrimTrailingEmptyCells удаляет пустые ячейки только с конца строки.
func TrimTrailingEmptyCells(record []string) []string {
	last := len(record)
	for last > 0 && CleanCell(record[last-1]) == "" {
		last--
	}

	return record[:last]
}

// IsEmptyRecord сообщает, что все значения строки пусты.
func IsEmptyRecord(record []string) bool {
	for _, value := range record {
		if CleanCell(value) != "" {
			return false
		}
	}

	return true
}

// RecordToMap сопоставляет значения строки с заголовками.
func RecordToMap(headers []string, record []string) map[string]string {
	row := make(map[string]string, len(headers))
	for index, header := range headers {
		value := ""
		if index < len(record) {
			value = CleanCell(record[index])
		}
		row[header] = value
	}

	return row
}

// CloneRow создаёт независимую копию строки.
func CloneRow(row map[string]string) map[string]string {
	clone := make(map[string]string, len(row))
	for key, value := range row {
		clone[key] = value
	}

	return clone
}

// ContainsString проверяет точное наличие строки в списке.
func ContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
