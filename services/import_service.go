// import_service.go - проверка локального черновика и read-only план импорта.
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task1/models"
	"task1/utils"
)

var (
	ErrInvalidImportID       = errors.New("Некорректный идентификатор импорта")
	ErrDraftHasInvalidRows   = errors.New("Исправьте ошибочные строки перед проверкой конфликтов")
	ErrDecisionMissing       = errors.New("Для всех конфликтов необходимо выбрать действие")
	ErrDecisionInvalid       = errors.New("Передано некорректное решение конфликта")
	ErrPreviewOutdated       = errors.New("Данные изменились после предпросмотра; проверьте конфликты повторно")
	ErrPhoneColumnNotFound   = errors.New(ErrorPhoneColNotFound)
	ErrImportMetadataInvalid = errors.New("Некорректные метаданные импортируемого файла")
)

// ImportContactReader ограничивает предпросмотр единственной read-only операцией PostgreSQL.
type ImportContactReader interface {
	FindContactsByPhones(ctx context.Context, phones []string) (map[string]models.Contact, error)
}

// NewImportValidation преобразует результат парсинга в локальный черновик.
// Вызов не обращается к PostgreSQL: ImportID нужен только для защиты финального commit.
func NewImportValidation(data models.FileData, importID string) models.ImportValidationResult {
	data.ID = importID
	return validationResult(data)
}

// ValidateImportDraft повторно нормализует данные, полученные из браузера.
// Сервер не доверяет клиентским stats и invalidRows, поэтому вычисляет их заново.
func ValidateImportDraft(draft models.ImportDraft) (models.FileData, error) {
	draft.ImportID = strings.TrimSpace(draft.ImportID)
	if !utils.IsUUID(draft.ImportID) {
		return models.FileData{}, ErrInvalidImportID
	}
	if draft.Size < 0 || draft.HeaderRow < 0 || draft.EmptyRowCount < 0 || draft.SkippedRowCount < 0 {
		return models.FileData{}, ErrImportMetadataInvalid
	}

	format := strings.ToLower(strings.TrimSpace(draft.Format))
	switch format {
	case "csv", "xls", "xlsx":
	default:
		return models.FileData{}, ErrImportMetadataInvalid
	}

	headers, err := normalizeHeaders(draft.Headers)
	if err != nil {
		return models.FileData{}, err
	}
	if len(draft.Rows) == 0 {
		return models.FileData{}, ErrNoDataRows
	}

	parsedRows := make([]parsedDataRow, 0, len(draft.Rows))
	seenRowNumbers := make(map[int]struct{}, len(draft.Rows))
	for index, draftRow := range draft.Rows {
		rowNumber := draftRow.RowNumber
		if rowNumber <= 0 {
			rowNumber = sourceDraftRowNumber(draft.HeaderRow, index)
		}
		if _, exists := seenRowNumbers[rowNumber]; exists {
			return models.FileData{}, fmt.Errorf("Повторяется номер строки %d", rowNumber)
		}
		seenRowNumbers[rowNumber] = struct{}{}

		values := make(map[string]string, len(headers))
		for headerIndex, header := range headers {
			sourceHeader := draft.Headers[headerIndex]
			values[header] = draftRow.Values[sourceHeader]
		}
		parsedRows = append(parsedRows, parsedDataRow{Number: rowNumber, Values: values})
	}

	invalidRows, warnings := validateRows(headers, parsedRows)
	data := models.FileData{
		ID:               draft.ImportID,
		OriginalFilename: strings.TrimSpace(draft.OriginalFilename),
		Size:             draft.Size,
		MIMEType:         strings.TrimSpace(draft.MIMEType),
		DetectedMIMEType: strings.TrimSpace(draft.DetectedMIMEType),
		Format:           format,
		Encoding:         strings.TrimSpace(draft.Encoding),
		SheetName:        strings.TrimSpace(draft.SheetName),
		Sheets:           append([]string(nil), draft.Sheets...),
		HeaderRow:        draft.HeaderRow,
		Headers:          headers,
		Warnings:         warnings,
		InvalidRows:      invalidRows,
		Stats: models.ProcessingStats{
			EmptyRowCount:   draft.EmptyRowCount,
			SkippedRowCount: draft.SkippedRowCount,
		},
	}
	for _, row := range parsedRows {
		data.Rows = append(data.Rows, row.Values)
		data.RowNumbers = append(data.RowNumbers, row.Number)
	}
	RefreshStats(&data)
	return data, nil
}

// ValidationResult возвращает безопасный HTTP-ответ после повторной проверки черновика.
func ValidationResult(data models.FileData) models.ImportValidationResult {
	return validationResult(data)
}

// PreviewImport сравнивает проверенные строки с текущими контактами без записи в БД.
func PreviewImport(ctx context.Context, reader ImportContactReader, data models.FileData) (models.ImportPreviewResult, error) {
	contacts, skipped, err := contactsFromFileData(data)
	if err != nil {
		return models.ImportPreviewResult{}, err
	}

	phones := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		phones = append(phones, contact.Phone)
	}
	existingByPhone, err := reader.FindContactsByPhones(ctx, phones)
	if err != nil {
		return models.ImportPreviewResult{}, fmt.Errorf("load contacts for import preview: %w", err)
	}

	result := models.ImportPreviewResult{SkippedCount: skipped}
	for _, incoming := range contacts {
		existing, exists := existingByPhone[incoming.Phone]
		switch {
		case !exists:
			result.NewCount++
		case ContactsEqual(existing, incoming):
			result.MatchedCount++
		default:
			result.Conflicts = append(result.Conflicts, detectConflict(incoming.SourceRow, existing, incoming))
		}
	}
	result.ConflictCount = len(result.Conflicts)
	return result, nil
}

// PrepareImport проверяет решения конфликтов и формирует данные для одной транзакции.
func PrepareImport(data models.FileData, preview models.ImportPreviewResult, decisions []models.ImportDecision) (
	[]models.Contact,
	map[string]models.ImportDecision,
	error,
) {
	contacts, _, err := contactsFromFileData(data)
	if err != nil {
		return nil, nil, err
	}

	conflicts := make(map[string]models.ConflictInfo, len(preview.Conflicts))
	for _, conflict := range preview.Conflicts {
		conflicts[conflict.Phone] = conflict
	}

	decisionByPhone := make(map[string]models.ImportDecision, len(decisions))
	for _, decision := range decisions {
		decision.Phone = strings.TrimSpace(decision.Phone)
		conflict, exists := conflicts[decision.Phone]
		if !exists || !validConflictAction(decision.Action) {
			return nil, nil, ErrDecisionInvalid
		}
		if decision.Version != conflict.Version {
			return nil, nil, ErrPreviewOutdated
		}
		if _, duplicate := decisionByPhone[decision.Phone]; duplicate {
			return nil, nil, ErrDecisionInvalid
		}
		decisionByPhone[decision.Phone] = decision
	}
	if len(decisionByPhone) != len(conflicts) {
		return nil, nil, ErrDecisionMissing
	}

	return contacts, decisionByPhone, nil
}

func contactsFromFileData(data models.FileData) ([]models.Contact, int, error) {
	if len(data.InvalidRows) > 0 {
		return nil, 0, ErrDraftHasInvalidRows
	}
	phoneColumn := utils.DetectPhoneColumn(data.Headers)
	if phoneColumn == "" {
		return nil, 0, ErrPhoneColumnNotFound
	}

	contacts := make([]models.Contact, 0, len(data.Rows))
	skipped := 0
	for index, row := range data.Rows {
		phone := strings.TrimSpace(row[phoneColumn])
		if phone == "" {
			skipped++
			continue
		}

		contact := RowToContact(row, phone, data.ID)
		contact.SourceRow = sourceDataRowNumber(data, index)
		contacts = append(contacts, contact)
	}
	return contacts, skipped, nil
}

func validationResult(data models.FileData) models.ImportValidationResult {
	draft := models.ImportDraft{
		ImportID:         data.ID,
		OriginalFilename: data.OriginalFilename,
		Size:             data.Size,
		MIMEType:         data.MIMEType,
		DetectedMIMEType: data.DetectedMIMEType,
		Format:           data.Format,
		Encoding:         data.Encoding,
		SheetName:        data.SheetName,
		Sheets:           append([]string(nil), data.Sheets...),
		HeaderRow:        data.HeaderRow,
		Headers:          append([]string(nil), data.Headers...),
		EmptyRowCount:    data.Stats.EmptyRowCount,
		SkippedRowCount:  data.Stats.SkippedRowCount,
	}
	for index, row := range data.Rows {
		draft.Rows = append(draft.Rows, models.ImportRow{
			RowNumber: sourceDataRowNumber(data, index),
			Values:    utils.CloneRow(row),
		})
	}

	previewLimit := 10
	if len(data.Rows) < previewLimit {
		previewLimit = len(data.Rows)
	}
	previewRows := make([]map[string]string, 0, previewLimit)
	for _, row := range data.Rows[:previewLimit] {
		previewRows = append(previewRows, utils.CloneRow(row))
	}

	return models.ImportValidationResult{
		Draft:               draft,
		PreviewRows:         previewRows,
		Stats:               data.Stats,
		Warnings:            append([]models.ProcessingWarning(nil), data.Warnings...),
		InvalidRows:         append([]models.InvalidRow(nil), data.InvalidRows...),
		DetectedPhoneColumn: utils.DetectPhoneColumn(data.Headers),
	}
}

func sourceDraftRowNumber(headerRow, index int) int {
	if headerRow > 0 {
		return headerRow + index + 1
	}
	return index + 1
}

func sourceDataRowNumber(data models.FileData, index int) int {
	if index < len(data.RowNumbers) && data.RowNumbers[index] > 0 {
		return data.RowNumbers[index]
	}
	return sourceDraftRowNumber(data.HeaderRow, index)
}

func validConflictAction(action models.ConflictAction) bool {
	switch action {
	case models.ConflictActionSkip, models.ConflictActionReplace, models.ConflictActionMerge:
		return true
	default:
		return false
	}
}
