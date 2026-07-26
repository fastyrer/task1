package services

import (
	"fmt"

	"task1/internal/contactdata"
	"task1/internal/models"
	"task1/internal/tabular"
)

// validateRows нормализует известные поля и собирает ошибки по исходным строкам.
func validateRows(headers []string, rows []parsedDataRow) ([]models.InvalidRow, []models.ProcessingWarning) {
	kinds := make(map[string]contactdata.ColumnKind, len(headers))
	for _, header := range headers {
		kinds[header] = contactdata.ClassifyHeader(header)
	}

	seenValues := make(map[string]map[string]int)
	invalidRows := make([]models.InvalidRow, 0)
	warnings := make([]models.ProcessingWarning, 0)
	for _, row := range rows {
		rowErrors := make([]models.ProcessingWarning, 0)
		for _, header := range headers {
			value := tabular.CleanCell(row.Values[header])
			row.Values[header] = value
			if value == "" {
				continue
			}

			switch kinds[header] {
			case contactdata.ColumnPhone:
				normalized, ok := contactdata.NormalizePhone(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный телефон."))
					continue
				}
				row.Values[header] = normalized
				rowErrors = append(rowErrors, duplicateWarning(seenValues, header, normalized, row.Number)...)
			case contactdata.ColumnEmail:
				normalized, ok := contactdata.NormalizeEmail(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный email."))
					continue
				}
				row.Values[header] = normalized
			case contactdata.ColumnDiscount:
				normalized, ok := contactdata.NormalizePercent(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Скидка должна быть числом от 0 до 100."))
					continue
				}
				row.Values[header] = normalized
			}
		}

		if len(rowErrors) > 0 {
			warnings = append(warnings, rowErrors...)
			invalidRows = append(invalidRows, models.InvalidRow{
				Row:    row.Number,
				Values: tabular.CloneRow(row.Values),
				Errors: rowErrors,
			})
		}
	}

	return invalidRows, warnings
}

// fieldWarning создаёт предупреждение для конкретной ячейки.
func fieldWarning(row int, column string, message string) models.ProcessingWarning {
	return models.ProcessingWarning{
		Row:     row,
		Column:  column,
		Message: message,
	}
}

// duplicateWarning сообщает номер первой строки с тем же значением колонки.
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

// RefreshStats пересчитывает статистику по текущему состоянию FileData.
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
