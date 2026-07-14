// rows.go – очистка и преобразование строк/ячеек из CSV и Excel.
//
// CleanHeader/CleanCell нормализуют отдельные значения. RecordToMap
// превращает строку CSV в map[заголовок]значение. Остальные функции
// помогают при парсинге: обрезка пустых ячеек, проверка пустых строк,
// клонирование map и поиск в слайсе.
package utils

import "strings"

// CleanHeader нормализует заголовок: удаляет BOM, неразрывные пробелы,
// схлопывает пробелы.
func CleanHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}

// CleanCell заменяет неразрывные пробелы и обрезает края.
func CleanCell(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.TrimSpace(value)
}

// TrimTrailingEmptyCells удаляет пустые ячейки с конца записи.
func TrimTrailingEmptyCells(record []string) []string {
	last := len(record)
	for last > 0 && CleanCell(record[last-1]) == "" {
		last--
	}
	return record[:last]
}

// IsEmptyRecord проверяет, что все значения строки пустые.
func IsEmptyRecord(record []string) bool {
	for _, v := range record {
		if CleanCell(v) != "" {
			return false
		}
	}
	return true
}

// RecordToMap преобразует строку CSV в map[заголовок]значение.
func RecordToMap(headers []string, record []string) map[string]string {
	row := make(map[string]string, len(headers))
	for i, h := range headers {
		v := ""
		if i < len(record) {
			v = CleanCell(record[i])
		}
		row[h] = v
	}
	return row
}

// CloneRow создаёт независимую копию map.
func CloneRow(row map[string]string) map[string]string {
	clone := make(map[string]string, len(row))
	for k, v := range row {
		clone[k] = v
	}
	return clone
}

// ContainsString проверяет наличие строки в слайсе.
func ContainsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
