// Package services содержит валидацию строк, проверку полей и обновление статистики.
//
// row_validator.go – проверяет значения в строках: нормализация телефонов и email,
// проверка процентов и дат, поиск дубликатов и формирование предупреждений/invalid rows.

package services

import (
	"fmt"

	"task1/models"
	"task1/utils"
)

// validateRows – проверяет каждую строку данных и возвращает список invalid-строк и предупреждений.

// validateRows:
// 1. Классифицирует колонки по типам через utils.ClassifyHeader
// 2. Для каждой строки очищает значения и нормализует их по типу
// 3. Накапливает предупреждения и отмечает invalid-строки
func validateRows(headers []string, rows []parsedDataRow) ([]models.InvalidRow, []models.ProcessingWarning) {

	// Классифицирует колонки по типам через utils.ClassifyHeader
	kinds := make(map[string]utils.ColumnKind, len(headers))
	for _, header := range headers {
		kinds[header] = utils.ClassifyHeader(header)
	}

	seenValues := make(map[string]map[string]int)
	invalidRows := make([]models.InvalidRow, 0)
	warnings := make([]models.ProcessingWarning, 0)
	for _, row := range rows {
		rowErrors := make([]models.ProcessingWarning, 0)
		for _, header := range headers {

			// Очистка значения
			value := utils.CleanCell(row.Values[header])
			row.Values[header] = value
			if value == "" {
				continue
			}

			// Нормализация и валидация по типу колонки
			switch kinds[header] {
			case utils.ColumnPhone:
				normalized, ok := utils.NormalizePhone(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный телефон."))
					continue
				}
				row.Values[header] = normalized
				rowErrors = append(rowErrors, duplicateWarning(seenValues, header, normalized, row.Number)...)
			case utils.ColumnEmail:
				normalized, ok := utils.NormalizeEmail(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный email."))
					continue
				}
				row.Values[header] = normalized
				rowErrors = append(rowErrors, duplicateWarning(seenValues, header, normalized, row.Number)...)
			case utils.ColumnDiscount:
				normalized, ok := utils.NormalizePercent(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Скидка должна быть числом от 0 до 100."))
					continue
				}
				row.Values[header] = normalized
			}
		}

		// Если были ошибки — помечаем строку как invalid и добавляем warnings
		if len(rowErrors) > 0 {
			warnings = append(warnings, rowErrors...)
			invalidRows = append(invalidRows, models.InvalidRow{
				Row:    row.Number,
				Values: utils.CloneRow(row.Values),
				Errors: rowErrors,
			})
		}
	}

	return invalidRows, warnings
}

// fieldWarning – вспомогательная функция для создания ProcessingWarning для конкретного поля.
func fieldWarning(row int, column string, message string) models.ProcessingWarning {
	return models.ProcessingWarning{
		Row:     row,
		Column:  column,
		Message: message,
	}
}

// duplicateWarning – проверяет дубликаты значений в колонке и возвращает предупреждение, если найдено.
func duplicateWarning(seenValues map[string]map[string]int, column string, value string, row int) []models.ProcessingWarning {
	if seenValues[column] == nil {
		seenValues[column] = make(map[string]int)
	}

	if firstRow, ok := seenValues[column][value]; ok {
		return []models.ProcessingWarning{
			fieldWarning(row, column, fmt.Sprintf("Дубликат значения; впервые встречено в строке %d.", firstRow)),
		}
	}

	seenValues[column][value] = row
	return nil
}

// RefreshStats – обновляет статистику в models.FileData по текущему состоянию.
func RefreshStats(data *models.FileData) {
	data.Stats.ColumnCount = len(data.Headers)
	data.Stats.RowCount = len(data.Rows)
	data.Stats.InvalidRowCount = len(data.InvalidRows)
	data.Stats.ValidRowCount = data.Stats.RowCount - data.Stats.InvalidRowCount
	if data.Stats.ValidRowCount < 0 {
		data.Stats.ValidRowCount = 0
	}
	data.Stats.WarningCount = len(data.Warnings)
}
