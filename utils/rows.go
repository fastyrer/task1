// rows.go – очистка и преобразование строк/ячеек из CSV и Excel.
//
// Функции работают при парсинге: нормализуют пробелы, отрезают BOM,
// отбрасывают пустые ячейки с хвоста, преобразуют строку в map.
package utils

import "strings"

// CleanHeader нормализует заголовок колонки.
//
// Преобразования:
//   - Удаление BOM (U+FEFF) — символ метки порядка байтов из Windows-блокнота
//   - Замена неразрывных пробелов (U+00A0) на обычные
//   - Схлопывание любых пробельных символов в один пробел (через Fields+Join)
//
// Примеры:
//
//	CleanHeader("\ufeffТелефон") → "Телефон"
//	CleanHeader("  Имя  ")       → "Имя"
//	CleanHeader("E–mail")        → "E-mail" (среднее тире не удаляется)
func CleanHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}

// CleanCell заменяет неразрывные пробелы и обрезает края.
//
// В Excel неразрывные пробелы (U+00A0, неразрывный пробел) часто попадают
// из скопированного текста. Обычный TrimSpace их не убирает,
// поэтому заменяем явно.
//
// Пример: CleanCell("  сумма\u00a0с НДС  ") → "сумма с НДС"
func CleanCell(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.TrimSpace(value)
}

// TrimTrailingEmptyCells удаляет пустые ячейки с конца строки-записи.
//
// CSV-парсер может вернуть строку вида ["a", "b", "", ""] когда в конце
// файла есть лишние разделители. Функция обрезает пустые ячейки только
// справа, не затрагивая пустые ячейки в середине.
//
// Пример: TrimTrailingEmptyCells(["a", "", "b", "", ""]) → ["a", "", "b"]
func TrimTrailingEmptyCells(record []string) []string {
	last := len(record)
	for last > 0 && CleanCell(record[last-1]) == "" {
		last--
	}

	return record[:last]
}

// IsEmptyRecord проверяет, что все значения строки пустые.
//
// Используется в парсерах для пропуска пустых строк.
// Строка из одних пробелов считается пустой.
func IsEmptyRecord(record []string) bool {
	for _, value := range record {
		if CleanCell(value) != "" {
			return false
		}
	}

	return true
}

// RecordToMap преобразует строку из CSV (слайс значений) в map[заголовок]значение.
//
// Если значений меньше, чем заголовков — недостающие заполняются пустой строкой.
// Все значения проходят CleanCell.
//
// Пример:
//
//	headers = ["Имя", "Телефон", "Город"]
//	record = ["Иван", "+7 912 345-67-89"]
//	→ {"Имя": "Иван", "Телефон": "+7 912 345-67-89", "Город": ""}
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

// CloneRow создаёт независимую копию map строки.
//
// В Go map — ссылочный тип. Простое присваивание не копирует данные.
// CloneRow нужна, чтобы сохранить снимок строки до её изменения
// (например, в validateRows для InvalidRows).
func CloneRow(row map[string]string) map[string]string {
	clone := make(map[string]string, len(row))
	for key, value := range row {
		clone[key] = value
	}

	return clone
}

// ContainsString проверяет точное наличие строки в слайсе.
//
// Пример: ContainsString(["A", "B", "C"], "B") → true
func ContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
