// Package services содержит бизнес-логику: парсинг файлов, валидацию, шаблоны.
//
// file_parser.go – чтение файлов, детекция формата (CSV/XLS/XLSX), выбор листа Excel,
// преобразование сырых строк в FileData. Первая непустая строка считается заголовками.

package services

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"task1/models"
	"task1/utils"
)

var (
	ErrUnsupportedFormat = errors.New("Неподдерживаемый формат файла. Загрузите CSV, XLS или XLSX.")
	ErrEmptyFile         = errors.New("Пустой файл.")
	ErrNoHeaders         = errors.New("В файле нет заголовков.")
	ErrNoDataRows        = errors.New("В файле нет строк с данными.")
	ErrInvalidCSV        = errors.New("Некорректная структура CSV.")
	ErrInvalidExcel      = errors.New("Некорректная структура XLS/XLSX.")
	ErrReadFile          = errors.New("Не удалось прочитать файл.")
	ErrFileTypeMismatch  = errors.New("Расширение файла не совпадает с содержимым.")
	ErrInvalidEncoding   = errors.New("Недопустимая кодировка")
	ErrSheetNotFound     = errors.New("Лист Excel не найден.")
)

// ParseOptions – параметры парсинга
type ParseOptions struct {
	SheetName string
}

// parsedRecord – вспомогательная структура для сырой записи из файла с номером строки и слайсом значений.
type parsedRecord struct {
	Number int
	Values []string
}

// parsedDataRow – внутреннее представление строки данных: номер исходной строки и мапа заголовок->значение.
type parsedDataRow struct {
	Number int
	Values map[string]string
}

// ParseByFilename – парсит файл по имени, автоматически определяя формат и обрабатывая опции.
func ParseByFilename(file multipart.File, filename string) (models.FileData, error) {
	return ParseByFilenameWithOptions(file, filename, ParseOptions{})
}

// ParseByFilenameWithOptions:
	// 1. Считывает содержимое файла
	// 2. Определяет формат по имени файла и содержимому
	// 3. Делегирует разбор в parseCSVContent или parseExcelContent
	// 4. Заполняет метаданные формата через setFileFormat
func ParseByFilenameWithOptions(file multipart.File, filename string, options ParseOptions) (models.FileData, error) {

	// 1. Считывает содержимое файла
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	// 2. Определяет формат по имени файла и содержимому
	format, err := detectFileFormat(filename, content)
	if err != nil {
		return models.FileData{}, err
	}

	var data models.FileData

	// 3. Делегирует разбор в parseCSVContent или parseExcelContent
	switch format {
	case "csv":
		data, err = parseCSVContent(content)
	case "xls", "xlsx":
		data, err = parseExcelContent(content, options)
	default:
		err = ErrUnsupportedFormat
	}
	if err != nil {
		return models.FileData{}, err
	}

	// 4. Заполняет метаданные формата через setFileFormat
	setFileFormat(&data, format, content)
	return data, nil
}

// ParseCSV – вспомогательный метод для парсинга CSV напрямую из multipart.File.

// ParseCSV:
	// 1. Считывает содержимое файла
	// 2. Вызывает parseCSVContent и устанавливает формат
func ParseCSV(file multipart.File) (models.FileData, error) {

	// 1. Считывает содержимое файла
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	// 2. Вызывает parseCSVContent и устанавливает формат
	data, err := parseCSVContent(content)
	if err != nil {
		return models.FileData{}, err
	}
	setFileFormat(&data, "csv", content)
	return data, nil
}

// ParseExcel – вспомогательный метод для парсинга Excel напрямую из multipart.File.

// ParseExcel:
	// 1. Считывает содержимое файла
	// 2. Вызывает parseExcelContent с пустыми опциями и устанавливает формат
func ParseExcel(file multipart.File) (models.FileData, error) {

	// 1. Считывает содержимое файла
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	// 2. Вызывает parseExcelContent с пустыми опциями и устанавливает формат
	data, err := parseExcelContent(content, ParseOptions{})
	if err != nil {
		return models.FileData{}, err
	}
	setFileFormat(&data, utils.ExcelFormat(content), content)
	return data, nil
}

// readFileContent – читает все байты из multipart.File и проверяет непустоту.

// readFileContent:
	// 1. Считывает весь файл в память
	// 2. Возвращает ErrReadFile при ошибке чтения
	// 3. Возвращает ErrEmptyFile при пустом содержимом
func readFileContent(file multipart.File) ([]byte, error) {
	
	// 1. Считывает весь файл в память
	content, err := io.ReadAll(file)
	
	// 2. Возвращает ErrReadFile при ошибке чтения
	if err != nil {
		return nil, ErrReadFile
	}

	// 3. Возвращает ErrEmptyFile при пустом содержимом
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, ErrEmptyFile
	}

	return content, nil
}

// detectFileFormat – определяет формат файла по расширению и проверяет соответствие содержимого.

// detectFileFormat:
	// 1. Сравнивает расширение файла
	// 2. Для CSV проверяет, не является ли содержимое Excel
	// 3. Для XLS/XLSX проверяет сигнатуру содержимого
	// 4. Возвращает ErrFileTypeMismatch или ErrUnsupportedFormat при несоответствии
func detectFileFormat(filename string, content []byte) (string, error) {
	// 1. Сравнивает расширение файла
	switch strings.ToLower(filepath.Ext(filename)) {
	
	// 2. Для CSV проверяет, не является ли содержимое Excel
	case ".csv":
		if utils.IsXLSX(content) || utils.IsXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "csv", nil

	// 3. Для XLS/XLSX проверяет сигнатуру содержимого
	case ".xls":
		if !utils.IsXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "xls", nil
	case ".xlsx":
		if !utils.IsXLSX(content) {
			return "", ErrFileTypeMismatch
		}
		return "xlsx", nil
	
	// 4. Возвращает ErrFileTypeMismatch или ErrUnsupportedFormat при несоответствии
	default:
		return "", ErrUnsupportedFormat
	}
}

// setFileFormat – заполняет поля формата и MIME в models.FileData.

// setFileFormat:
	// 1. Устанавливает data.Format
	// 2. Вычисляет и сохраняет DetectedMIMEType
func setFileFormat(data *models.FileData, format string, content []byte) {
	data.Format = format
	data.DetectedMIMEType = utils.DetectedMIMEType(format, content)
}

// rowsToFileData – преобразует [][]string в models.FileData через recordsToFileData.

// rowsToFileData:
	// 1. Формирует parsedRecord с номерами строк
	// 2. Делегирует преобразование в recordsToFileData
func rowsToFileData(rows [][]string) (models.FileData, error) {

	// 1. Формирует parsedRecord с номерами строк
	records := make([]parsedRecord, 0, len(rows))
	for i, row := range rows {
		records = append(records, parsedRecord{
			Number: i + 1,
			Values: row,
		})
	}

	// 2. Делегирует преобразование в recordsToFileData
	return recordsToFileData(records)
}

// recordsToFileData – преобразует parsedRecord в models.FileData.
//
// Первая непустая строка считается заголовками. Все строки до неё
// помечаются как пропущенные (warning). Пустые строки после заголовков
// пропускаются без предупреждения.
func recordsToFileData(records []parsedRecord) (models.FileData, error) {
	if len(records) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	// Ищем первую непустую строку — это заголовки
	headerIndex := 0
	for headerIndex < len(records) {
		if !utils.IsEmptyRecord(records[headerIndex].Values) {
			break
		}
		headerIndex++
	}
	if headerIndex >= len(records) {
		return models.FileData{}, ErrNoHeaders
	}

	headers, err := normalizeHeaders(records[headerIndex].Values)
	if err != nil {
		return models.FileData{}, err
	}

	// Предупреждения о пропущенных строках до заголовков
	warnings := make([]models.ProcessingWarning, 0)
	for index := 0; index < headerIndex; index++ {
		if !utils.IsEmptyRecord(records[index].Values) {
			warnings = append(warnings, models.ProcessingWarning{
				Row:     records[index].Number,
				Message: "Строка пропущена до найденных заголовков.",
			})
		}
	}

	// Сбор данных после заголовков
	dataRows := make([]parsedDataRow, 0, len(records)-headerIndex-1)
	emptyRowCount := 0
	for _, record := range records[headerIndex+1:] {
		values := utils.TrimTrailingEmptyCells(record.Values)
		if utils.IsEmptyRecord(values) {
			emptyRowCount++
			continue
		}

		rowWarnings := rowShapeWarnings(record.Number, headers, values)
		warnings = append(warnings, rowWarnings...)
		dataRows = append(dataRows, parsedDataRow{
			Number: record.Number,
			Values: utils.RecordToMap(headers, values),
		})
	}

	if len(dataRows) == 0 {
		return models.FileData{}, ErrNoDataRows
	}

	invalidRows, validationWarnings := validateRows(headers, dataRows)
	warnings = append(warnings, validationWarnings...)

	rows := make([]map[string]string, 0, len(dataRows))
	rowNumbers := make([]int, 0, len(dataRows))
	for _, row := range dataRows {
		rows = append(rows, row.Values)
		rowNumbers = append(rowNumbers, row.Number)
	}

	data := models.FileData{
		HeaderRow:   records[headerIndex].Number,
		Headers:     headers,
		Rows:        rows,
		RowNumbers:  rowNumbers,
		Warnings:    warnings,
		InvalidRows: invalidRows,
		Stats: models.ProcessingStats{
			EmptyRowCount:   emptyRowCount,
			SkippedRowCount: headerIndex,
		},
	}
	RefreshStats(&data)

	return data, nil
}

// normalizeHeaders – очищает заголовки, проверяет на пустоту и дубликаты.
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

		key := strings.ToLower(header)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}
		seen[key] = struct{}{}
		headers[index] = header
	}

	return headers, nil
}

// rowShapeWarnings – проверяет форму строки относительно заголовков.
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
