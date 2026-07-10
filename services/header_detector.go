package services

import (
	"errors"
	"fmt"

	"task1/models"
	"task1/utils"
)

const maxHeaderScanRows = 25

func detectHeaderRecord(records []parsedRecord) (int, []string, []models.ProcessingWarning, error) {
	limit := len(records)
	if limit > maxHeaderScanRows {
		limit = maxHeaderScanRows
	}

	bestIndex := -1
	bestScore := -1 << 30
	var bestHeaders []string
	for index := 0; index < limit; index++ {
		values := utils.TrimTrailingEmptyCells(records[index].Values)
		if utils.IsEmptyRecord(values) {
			continue
		}

		headers, err := normalizeHeaders(values)
		if err != nil {
			continue
		}

		score := scoreHeaderCandidate(headers, records, index)
		if score > bestScore {
			bestIndex = index
			bestScore = score
			bestHeaders = headers
		}
	}

	if bestIndex == -1 {
		for index, record := range records {
			if utils.IsEmptyRecord(record.Values) {
				continue
			}
			headers, err := normalizeHeaders(record.Values)
			if err != nil {
				return 0, nil, nil, err
			}
			return index, headers, skippedBeforeHeaderWarnings(records, index), nil
		}
		return 0, nil, nil, ErrNoHeaders
	}

	return bestIndex, bestHeaders, skippedBeforeHeaderWarnings(records, bestIndex), nil
}

func scoreHeaderCandidate(headers []string, records []parsedRecord, index int) int {
	score := len(headers) * 3
	for _, header := range headers {
		if utils.IsCommonHeader(header) {
			score += 6
		}
		if utils.IsNumberLike(header) {
			score -= 3
		}
	}
	if hasDataAfter(records, index) {
		score += 5
	}

	return score - index
}

func hasDataAfter(records []parsedRecord, index int) bool {
	for _, record := range records[index+1:] {
		if !utils.IsEmptyRecord(record.Values) {
			return true
		}
	}

	return false
}

func skippedBeforeHeaderWarnings(records []parsedRecord, headerIndex int) []models.ProcessingWarning {
	if headerIndex == 0 {
		return nil
	}

	warnings := make([]models.ProcessingWarning, 0, headerIndex)
	for _, record := range records[:headerIndex] {
		if utils.IsEmptyRecord(record.Values) {
			continue
		}
		warnings = append(warnings, models.ProcessingWarning{
			Row:     record.Number,
			Message: "Строка пропущена до найденных заголовков.",
		})
	}

	return warnings
}

func rowShapeWarnings(rowNumber int, headers []string, values []string) []models.ProcessingWarning {
	warnings := make([]models.ProcessingWarning, 0, 2)
	if len(values) < len(headers) {
		warnings = append(warnings, models.ProcessingWarning{
			Row:     rowNumber,
			Message: "В строке меньше значений, чем заголовков; недостающие ячейки заполнены пустыми значениями.",
		})
	}
	if len(values) > len(headers) && !utils.IsEmptyRecord(values[len(headers):]) {
		warnings = append(warnings, models.ProcessingWarning{
			Row:     rowNumber,
			Message: "В строке есть лишние значения без заголовков; они не были сохранены.",
		})
	}

	return warnings
}

func normalizeHeaders(record []string) ([]string, error) {
	record = utils.TrimTrailingEmptyCells(record)
	if len(record) == 0 || utils.IsEmptyRecord(record) {
		return nil, ErrNoHeaders
	}

	headers := make([]string, len(record))
	seen := make(map[string]struct{}, len(record))
	for index, header := range record {
		header = utils.CleanHeader(header)
		if header == "" {
			return nil, errors.New("В файле есть пустые заголовки.")
		}

		key := utils.HeaderKey(header)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}

		seen[key] = struct{}{}
		headers[index] = header
	}

	return headers, nil
}
