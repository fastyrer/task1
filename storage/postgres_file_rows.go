// postgres_file_rows.go - подготовка проверенных строк для PostgreSQL COPY.
package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"task1/models"
)

// prepareFileRows формирует значения file_rows только для финального CommitImport.
func prepareFileRows(data models.FileData, fileID string) ([][]any, error) {
	invalidByRow := make(map[int][]models.ProcessingWarning, len(data.InvalidRows))
	for _, invalid := range data.InvalidRows {
		invalidByRow[invalid.Row] = invalid.Errors
	}

	rows := make([][]any, 0, len(data.Rows))
	for index, values := range data.Rows {
		if values == nil {
			values = map[string]string{}
		}
		rowNumber := sourceRowNumber(data, index)
		rowErrors := invalidByRow[rowNumber]

		valuesJSON, err := json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("marshal confirmed row %d: %w", rowNumber, err)
		}
		errorsJSON, err := marshalSlice(rowErrors)
		if err != nil {
			return nil, fmt.Errorf("marshal confirmed row errors %d: %w", rowNumber, err)
		}

		rows = append(rows, []any{
			fileID,
			index + 1,
			rowNumber,
			valuesJSON,
			len(rowErrors) == 0,
			errorsJSON,
			rowSearchText(values),
		})
	}
	return rows, nil
}

// rowSearchText сохраняет производное поисковое представление только подтверждённой строки.
func rowSearchText(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	searchValues := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			searchValues = append(searchValues, value)
		}
	}
	return strings.Join(searchValues, "\n")
}

func sourceRowNumber(data models.FileData, index int) int {
	if index < len(data.RowNumbers) && data.RowNumbers[index] > 0 {
		return data.RowNumbers[index]
	}
	if data.HeaderRow > 0 {
		return data.HeaderRow + index + 1
	}
	return index + 1
}

// marshalSlice гарантирует JSON-массив вместо null для NOT NULL JSONB-полей.
func marshalSlice[T any](values []T) ([]byte, error) {
	if values == nil {
		values = []T{}
	}
	return json.Marshal(values)
}
