// Package services содержит парсеры файлов и преобразование их содержимого в models.FileData.
//
// csv_parser.go – отвечает за разбор CSV-контента: декодирование по кодировке,
// определение разделителя, чтение записей и преобразование их в структуру данных.

package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"task1/models"
	"task1/utils"
)

// parseCSVContent – декодирует CSV-байты, определяет разделитель и парсит записи в models.FileData.
//
// parseCSVContent:
	// 1. Декодирование содержимого и определение кодировки
	// 2. Проверка на пустой файл
	// 3. Настройка CSV-ридера (разделитель, пробелы, число полей)
	// 4. Чтение всех записей в слайс parsedRecord
	// 5. Преобразование parsedRecord в models.FileData через recordsToFileData
	// 6. Установка поля Encoding и добавление предупреждения, если кодировка не UTF-8
func parseCSVContent(content []byte) (models.FileData, error) {
	// 1. Декодирование содержимого и определение кодировки
	text, encoding, err := utils.DecodeCSVContent(content)
	if err != nil {
		return models.FileData{}, err
	}

	// 2. Проверка на пустой файл
	if strings.TrimSpace(text) == "" {
		return models.FileData{}, ErrEmptyFile
	}

	// 3. Настройка CSV-ридера (разделитель, пробелы, число полей)
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.Comma = utils.DetectCSVDelimiter(text)

	// 4. Чтение всех записей в слайс parsedRecord
	records := make([]parsedRecord, 0)
	recordNumber := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		recordNumber++
		if err != nil {
			return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidCSV, err)
		}

		records = append(records, parsedRecord{
			Number: recordNumber,
			Values: record,
		})
	}

	// 5. Преобразование parsedRecord в models.FileData через recordsToFileData
	data, err := recordsToFileData(records)
	if err != nil {
		return models.FileData{}, err
	}

	// 6. Установка поля Encoding и добавление предупреждения, если кодировка не UTF-8
	data.Encoding = encoding
	if encoding != "UTF-8" {
		data.Warnings = append(data.Warnings, models.ProcessingWarning{
			Message: fmt.Sprintf("CSV декодирован из %s.", encoding),
		})
		RefreshStats(&data)
	}

	return data, nil
}
