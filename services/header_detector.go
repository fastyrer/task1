// Package services содержит утилиты для обнаружения заголовков и предупреждений при парсинге файлов.
//
// header_detector.go – ищет строку заголовков в наборе сырых записей, оценивает кандидатов
// и формирует предупреждения для пропущенных или некорректных строк.

package services

import (
	"errors"
	"fmt"

	"task1/models"
	"task1/utils"
)

// maxHeaderScanRows – максимальное число строк для сканирования заголовков в начале файла.
const maxHeaderScanRows = 25

// detectHeaderRecord – находит наиболее вероятную строку заголовков и возвращает её индекс,
// заголовки, накопленные предупреждения и ошибку при отсутствии заголовков.

// detectHeaderRecord:
	// 1. Ограничивает число исследуемых строк до maxHeaderScanRows
	// 2. Нормализует кандидатов в заголовки и вычисляет для них скор
	// 3. Выбирает лучший кандидат по скору
	// 4. Если кандидат не найден, пытается найти любой валидный набор заголовков дальше по файлу
func detectHeaderRecord(records []parsedRecord) (int, []string, []models.ProcessingWarning, error) {

	// 1. Ограничивает число исследуемых строк до maxHeaderScanRows
	limit := len(records)
	if limit > maxHeaderScanRows {
		limit = maxHeaderScanRows
	}

	bestIndex := -1
	bestScore := -1 << 30
	var bestHeaders []string

	// 2. Нормализует кандидатов в заголовки и вычисляет для них скор
	for index := 0; index < limit; index++ {

		values := utils.TrimTrailingEmptyCells(records[index].Values)
		if utils.IsEmptyRecord(values) {
			continue
		}

		headers, err := normalizeHeaders(values)
		if err != nil {
			continue
		}

		// 3. Выбирает лучший кандидат по скору
		score := scoreHeaderCandidate(headers, records, index)
		if score > bestScore {
			bestIndex = index
			bestScore = score
			bestHeaders = headers
		}
	}
	// 4. Если кандидат не найден, пытается найти любой валидный набор заголовков дальше по файлу
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



//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
// может лучше перепишем на фиксированную сторку и там берем хеддеры? (по первой строке просто будет проще, но не всегда верно) - пока оставим так, потом обсудим

// scoreHeaderCandidate – вычисляет оценку для кандидата в заголовки на основе набора правил.

// scoreHeaderCandidate:
	// 1. Базовый вес пропорционален числу заголовков
	// 2. Дополнительные баллы за известные общие заголовки
	// 3. Штрафы за числовые заголовки и бонус, если после строки есть данные
	// 4. Минус индекс для предпочтения более ранних кандидатов
func scoreHeaderCandidate(headers []string, records []parsedRecord, index int) int {
	// 1. Базовый вес пропорционален числу заголовков
	score := len(headers) * 3
	for _, header := range headers {
		// 2. Дополнительные баллы за известные общие заголовки
		if utils.IsCommonHeader(header) {
			score += 6
		}
		// 3. Штрафы за числовые заголовки и бонус, если после строки есть данные
		if utils.IsNumberLike(header) {
			score -= 3
		}
	}
	if hasDataAfter(records, index) {
		score += 5
	}
	// 4. Минус индекс для предпочтения более ранних кандидатов
	return score - index
}

// hasDataAfter – проверяет, есть ли непустые строки после указанного индекса.
func hasDataAfter(records []parsedRecord, index int) bool {
	for _, record := range records[index+1:] {
		if !utils.IsEmptyRecord(record.Values) {
			return true
		}
	}

	return false
}

// skippedBeforeHeaderWarnings – формирует предупреждения для строк до найденной строки заголовков.
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

// rowShapeWarnings – проверяет форму строки относительно заголовков и возвращает предупреждения.
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

// normalizeHeaders – очищает и проверяет заголовки, возвращает ошибку при пустых или дубликатных заголовках.

// normalizeHeaders:
	// 1. Обрезает пустые хвостовые ячейки
	// 2. Очищает каждый заголовок и проверяет на пустоту
	// 3. Проверяет на повторяющиеся ключи заголовков
func normalizeHeaders(record []string) ([]string, error) {
	// 1. Убираем пустые хвостовые ячейки
	record = utils.TrimTrailingEmptyCells(record)
	if len(record) == 0 || utils.IsEmptyRecord(record) {
		return nil, ErrNoHeaders
	}

	// 2. Очищаем и проверяем каждый заголовок, собирая результат
	headers := make([]string, len(record))
	seen := make(map[string]struct{}, len(record))
	for index, header := range record {
		// Очистка заголовка от лишних символов/пробелов
		header = utils.CleanHeader(header)
		if header == "" {
			return nil, errors.New("В файле есть пустые заголовки.")
		}

		// 3. Проверка на дубликаты по ключу заголовка
		key := utils.HeaderKey(header)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}

		seen[key] = struct{}{}
		headers[index] = header
	}

	return headers, nil
}
