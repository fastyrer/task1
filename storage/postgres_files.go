// postgres_files.go - хранение метаданных загруженного файла,
// его строк и поиск по содержимому средствами PostgreSQL.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"task1/models"
)

// SaveFileData - сохраняет файл и все его строки атомарно.
//
// Алгоритм:
//  1. Использует готовый ID или генерирует UUID.
//  2. Кодирует сложные поля в JSONB.
//  3. Готовит отдельные записи для file_rows.
//  4. В одной транзакции обновляет метаданные и заменяет набор строк.
//  5. Для массовой вставки использует PostgreSQL COPY.
func (s *PostgresStorage) SaveFileData(ctx context.Context, data models.FileData) (string, error) {
	const upsertFileQuery = `
		INSERT INTO uploaded_files (
			id,
			original_filename,
			size,
			mime_type,
			detected_mime_type,
			format,
			encoding,
			sheet_name,
			sheets,
			header_row,
			headers,
			row_count,
			column_count,
			stats,
			warnings
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			original_filename = EXCLUDED.original_filename,
			size = EXCLUDED.size,
			mime_type = EXCLUDED.mime_type,
			detected_mime_type = EXCLUDED.detected_mime_type,
			format = EXCLUDED.format,
			encoding = EXCLUDED.encoding,
			sheet_name = EXCLUDED.sheet_name,
			sheets = EXCLUDED.sheets,
			header_row = EXCLUDED.header_row,
			headers = EXCLUDED.headers,
			row_count = EXCLUDED.row_count,
			column_count = EXCLUDED.column_count,
			stats = EXCLUDED.stats,
			warnings = EXCLUDED.warnings
	`
	const deleteFileRowsQuery = `DELETE FROM file_rows WHERE file_id = $1`

	fileID := strings.TrimSpace(data.ID)
	if fileID == "" {
		var err error
		fileID, err = generateID()
		if err != nil {
			return "", err
		}
	}
	data.ID = fileID

	sheetsJSON, err := marshalSlice(data.Sheets)
	if err != nil {
		return "", fmt.Errorf("marshal file sheets: %w", err)
	}
	headersJSON, err := marshalSlice(data.Headers)
	if err != nil {
		return "", fmt.Errorf("marshal file headers: %w", err)
	}
	statsJSON, err := json.Marshal(data.Stats)
	if err != nil {
		return "", fmt.Errorf("marshal file stats: %w", err)
	}
	warningsJSON, err := marshalSlice(data.Warnings)
	if err != nil {
		return "", fmt.Errorf("marshal file warnings: %w", err)
	}

	copyRows, err := prepareFileRows(data, fileID)
	if err != nil {
		return "", err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return "", fmt.Errorf("begin save file transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	if _, err := tx.Exec(queryCtx, upsertFileQuery,
		fileID,
		data.OriginalFilename,
		data.Size,
		data.MIMEType,
		data.DetectedMIMEType,
		data.Format,
		data.Encoding,
		data.SheetName,
		sheetsJSON,
		data.HeaderRow,
		headersJSON,
		data.Stats.RowCount,
		data.Stats.ColumnCount,
		statsJSON,
		warningsJSON,
	); err != nil {
		return "", fmt.Errorf("save uploaded file: %w", err)
	}

	if _, err := tx.Exec(queryCtx, deleteFileRowsQuery, fileID); err != nil {
		return "", fmt.Errorf("replace file rows: %w", err)
	}
	if len(copyRows) > 0 {
		if _, err := tx.CopyFrom(
			queryCtx,
			pgx.Identifier{"file_rows"},
			[]string{"file_id", "position", "row_number", "values", "is_valid", "errors", "search_text"},
			pgx.CopyFromRows(copyRows),
		); err != nil {
			return "", fmt.Errorf("copy file rows: %w", err)
		}
	}

	if err := tx.Commit(queryCtx); err != nil {
		return "", fmt.Errorf("commit save file transaction: %w", err)
	}
	return fileID, nil
}

// GetFileData - восстанавливает FileData из двух таблиц.
// Сначала читает uploaded_files, затем по позиции собирает file_rows.
// Исходные номера строк и ошибки возвращаются без потерь.
func (s *PostgresStorage) GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error) {
	const getFileQuery = `
		SELECT
			original_filename,
			size,
			mime_type,
			detected_mime_type,
			format,
			encoding,
			sheet_name,
			sheets,
			header_row,
			headers,
			stats,
			warnings
		FROM uploaded_files
		WHERE id = $1
	`
	const getFileRowsQuery = `
		SELECT row_number, values, is_valid, errors
		FROM file_rows
		WHERE file_id = $1
		ORDER BY position
	`

	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return models.FileData{}, false, nil
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	data := models.FileData{ID: fileID}
	var sheetsJSON, headersJSON, statsJSON, warningsJSON []byte
	err := s.pool.QueryRow(queryCtx, getFileQuery, fileID).Scan(
		&data.OriginalFilename,
		&data.Size,
		&data.MIMEType,
		&data.DetectedMIMEType,
		&data.Format,
		&data.Encoding,
		&data.SheetName,
		&sheetsJSON,
		&data.HeaderRow,
		&headersJSON,
		&statsJSON,
		&warningsJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.FileData{}, false, nil
	}
	if err != nil {
		return models.FileData{}, false, fmt.Errorf("get uploaded file: %w", err)
	}

	if err := unmarshalJSON(sheetsJSON, &data.Sheets, "file sheets"); err != nil {
		return models.FileData{}, false, err
	}
	if err := unmarshalJSON(headersJSON, &data.Headers, "file headers"); err != nil {
		return models.FileData{}, false, err
	}
	if err := unmarshalJSON(statsJSON, &data.Stats, "file stats"); err != nil {
		return models.FileData{}, false, err
	}
	if err := unmarshalJSON(warningsJSON, &data.Warnings, "file warnings"); err != nil {
		return models.FileData{}, false, err
	}

	rows, err := s.pool.Query(queryCtx, getFileRowsQuery, fileID)
	if err != nil {
		return models.FileData{}, false, fmt.Errorf("get file rows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rowNumber int
		var valuesJSON, errorsJSON []byte
		var isValid bool
		if err := rows.Scan(&rowNumber, &valuesJSON, &isValid, &errorsJSON); err != nil {
			return models.FileData{}, false, fmt.Errorf("scan file row: %w", err)
		}

		values := make(map[string]string)
		if err := unmarshalJSON(valuesJSON, &values, "file row values"); err != nil {
			return models.FileData{}, false, err
		}
		data.Rows = append(data.Rows, values)
		data.RowNumbers = append(data.RowNumbers, rowNumber)

		if !isValid {
			var rowErrors []models.ProcessingWarning
			if err := unmarshalJSON(errorsJSON, &rowErrors, "file row errors"); err != nil {
				return models.FileData{}, false, err
			}
			data.InvalidRows = append(data.InvalidRows, models.InvalidRow{
				Row:    rowNumber,
				Values: values,
				Errors: rowErrors,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return models.FileData{}, false, fmt.Errorf("iterate file rows: %w", err)
	}

	return data, true, nil
}

// SearchFileRows - выполняет регистронезависимый поиск по search_text.
// Весь файл не загружается в Go: фильтрация, подсчёт и LIMIT работают в БД.
// Символы %, _ и \ экранируются, поэтому считаются обычным текстом.
func (s *PostgresStorage) SearchFileRows(ctx context.Context, fileID, query string, limit int) (models.FileSearchResult, bool, error) {
	const getSearchHeadersQuery = `
		SELECT headers
		FROM uploaded_files
		WHERE id = $1
	`
	const searchFileRowsQuery = `
		SELECT row_number, values, count(*) OVER()
		FROM file_rows
		WHERE file_id = $1
		  AND search_text ILIKE $2 ESCAPE '\'
		ORDER BY position
		LIMIT $3
	`

	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return models.FileSearchResult{}, false, nil
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	result := models.FileSearchResult{}
	var headersJSON []byte
	err := s.pool.QueryRow(queryCtx, getSearchHeadersQuery, fileID).Scan(&headersJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.FileSearchResult{}, false, nil
	}
	if err != nil {
		return models.FileSearchResult{}, false, fmt.Errorf("get search file: %w", err)
	}
	if err := unmarshalJSON(headersJSON, &result.Headers, "search headers"); err != nil {
		return models.FileSearchResult{}, false, err
	}

	pattern := "%" + escapeLikePattern(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(queryCtx, searchFileRowsQuery, fileID, pattern, limit)
	if err != nil {
		return models.FileSearchResult{}, false, fmt.Errorf("search file rows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var storedRow models.StoredFileRow
		var valuesJSON []byte
		var total int64
		if err := rows.Scan(&storedRow.Row, &valuesJSON, &total); err != nil {
			return models.FileSearchResult{}, false, fmt.Errorf("scan search row: %w", err)
		}
		storedRow.Values = make(map[string]string)
		if err := unmarshalJSON(valuesJSON, &storedRow.Values, "search row values"); err != nil {
			return models.FileSearchResult{}, false, err
		}
		result.Total = int(total)
		result.Rows = append(result.Rows, storedRow)
	}
	if err := rows.Err(); err != nil {
		return models.FileSearchResult{}, false, fmt.Errorf("iterate search rows: %w", err)
	}

	return result, true, nil
}

// prepareFileRows - преобразует FileData.Rows в набор значений для pgx.CopyFrom.
// Для каждой строки также собирается search_text и признак валидности.
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
			return nil, fmt.Errorf("marshal file row %d: %w", rowNumber, err)
		}
		errorsJSON, err := marshalSlice(rowErrors)
		if err != nil {
			return nil, fmt.Errorf("marshal file row errors %d: %w", rowNumber, err)
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

// rowSearchText собирает стабильную поисковую строку из всех значений строки файла.
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

// sourceRowNumber - возвращает номер строки из исходного файла.
// Если RowNumbers нет, номер вычисляется от строки заголовка.
func sourceRowNumber(data models.FileData, index int) int {
	if index < len(data.RowNumbers) && data.RowNumbers[index] > 0 {
		return data.RowNumbers[index]
	}
	if data.HeaderRow > 0 {
		return data.HeaderRow + index + 1
	}
	return index + 1
}

// marshalSlice - кодирует slice в JSON; nil сохраняется как [], а не null.
func marshalSlice[T any](values []T) ([]byte, error) {
	if values == nil {
		values = []T{}
	}
	return json.Marshal(values)
}

// unmarshalJSON - декодирует JSONB и добавляет к ошибке имя читаемого поля.
func unmarshalJSON(data []byte, target any, label string) error {
	if len(data) == 0 {
		return fmt.Errorf("decode %s: empty JSON", label)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

// escapeLikePattern - экранирует служебные символы LIKE для буквального поиска.
func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
