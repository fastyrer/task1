package services

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"task1/internal/contactdata"
	"task1/internal/models"
	"task1/internal/tabular"
)

var (
	// ErrUnsupportedFormat сообщает о неподдерживаемом расширении или формате.
	ErrUnsupportedFormat = errors.New("Неподдерживаемый формат файла. Загрузите CSV, XLS или XLSX.")
	// ErrEmptyFile сообщает, что файл не содержит данных.
	ErrEmptyFile = errors.New("Пустой файл.")
	// ErrNoHeaders сообщает об отсутствии строки заголовков.
	ErrNoHeaders = errors.New("В файле нет заголовков.")
	// ErrNoDataRows сообщает об отсутствии строк после заголовка.
	ErrNoDataRows = errors.New("В файле нет строк с данными.")
	// ErrInvalidCSV сообщает о некорректной структуре CSV.
	ErrInvalidCSV = errors.New("Некорректная структура CSV.")
	// ErrInvalidExcel сообщает о некорректной структуре книги Excel.
	ErrInvalidExcel = errors.New("Некорректная структура XLS/XLSX.")
	// ErrReadFile сообщает об ошибке чтения загруженного файла.
	ErrReadFile = errors.New("Не удалось прочитать файл.")
	// ErrFileTypeMismatch сообщает о несовпадении расширения и сигнатуры.
	ErrFileTypeMismatch = errors.New("Расширение файла не совпадает с содержимым.")
	// ErrInvalidEncoding сообщает о неподдерживаемой кодировке CSV.
	ErrInvalidEncoding = tabular.ErrInvalidCSVEncoding
	// ErrSheetNotFound сообщает об отсутствующем Excel-листе.
	ErrSheetNotFound = errors.New("Лист Excel не найден.")
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

// ParseByFilenameWithOptions читает файл, проверяет его сигнатуру и разбирает
// CSV/XLS/XLSX с учётом выбранного Excel-листа.
func ParseByFilenameWithOptions(file multipart.File, filename string, options ParseOptions) (models.FileData, error) {
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	format, err := detectFileFormat(filename, content)
	if err != nil {
		return models.FileData{}, err
	}

	var data models.FileData

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

	setFileFormat(&data, format, content)
	return data, nil
}

// readFileContent читает файл целиком и отклоняет пустое содержимое.
func readFileContent(file multipart.File) ([]byte, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, ErrReadFile
	}

	if len(bytes.TrimSpace(content)) == 0 {
		return nil, ErrEmptyFile
	}

	return content, nil
}

// detectFileFormat проверяет соответствие расширения сигнатуре содержимого.
func detectFileFormat(filename string, content []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		if tabular.IsXLSX(content) || tabular.IsXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "csv", nil

	case ".xls":
		if !tabular.IsXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "xls", nil
	case ".xlsx":
		if !tabular.IsXLSX(content) {
			return "", ErrFileTypeMismatch
		}
		return "xlsx", nil

	default:
		return "", ErrUnsupportedFormat
	}
}

// setFileFormat сохраняет проверенный формат и MIME-тип.
func setFileFormat(data *models.FileData, format string, content []byte) {
	data.Format = format
	data.DetectedMIMEType = tabular.DetectedMIMEType(format, content)
}

// rowsToFileData добавляет исходные номера строк и делегирует общий разбор.
func rowsToFileData(rows [][]string) (models.FileData, error) {
	records := make([]parsedRecord, 0, len(rows))
	for i, row := range rows {
		records = append(records, parsedRecord{
			Number: i + 1,
			Values: row,
		})
	}

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

	headerIndex := 0
	for headerIndex < len(records) {
		if !tabular.IsEmptyRecord(records[headerIndex].Values) {
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

	warnings := make([]models.ProcessingWarning, 0)
	for index := 0; index < headerIndex; index++ {
		if !tabular.IsEmptyRecord(records[index].Values) {
			warnings = append(warnings, models.ProcessingWarning{
				Row:     records[index].Number,
				Message: "Строка пропущена до найденных заголовков.",
			})
		}
	}

	dataRows := make([]parsedDataRow, 0, len(records)-headerIndex-1)
	emptyRowCount := 0
	for _, record := range records[headerIndex+1:] {
		values := tabular.TrimTrailingEmptyCells(record.Values)
		if tabular.IsEmptyRecord(values) {
			emptyRowCount++
			continue
		}

		rowWarnings := rowShapeWarnings(record.Number, headers, values)
		warnings = append(warnings, rowWarnings...)
		dataRows = append(dataRows, parsedDataRow{
			Number: record.Number,
			Values: tabular.RecordToMap(headers, values),
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

// normalizeHeaders очищает заголовки и отклоняет неоднозначную структуру.
// Помимо текстовых дубликатов запрещены несколько колонок одного контактного
// типа: модель contacts может однозначно хранить только один телефон, email,
// ФИО и скидку.
func normalizeHeaders(record []string) ([]string, error) {
	record = tabular.TrimTrailingEmptyCells(record)
	if len(record) == 0 || tabular.IsEmptyRecord(record) {
		return nil, ErrNoHeaders
	}

	headers := make([]string, len(record))
	seen := make(map[string]struct{}, len(record))
	contactHeaders := make(map[contactdata.ColumnKind]string, 4)
	for index, header := range record {
		header = tabular.CleanHeader(header)
		if header == "" {
			return nil, errors.New("В файле есть пустые заголовки.")
		}

		key := strings.ToLower(header)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}
		seen[key] = struct{}{}

		kind := contactdata.ClassifyHeader(header)
		if kind != contactdata.ColumnGeneric {
			if previous, exists := contactHeaders[kind]; exists {
				return nil, fmt.Errorf(
					"Колонки %q и %q описывают одно поле контакта; оставьте только одну из них.",
					previous,
					header,
				)
			}
			contactHeaders[kind] = header
		}
		headers[index] = header
	}

	return headers, nil
}

// rowShapeWarnings проверяет форму строки относительно заголовков.
func rowShapeWarnings(rowNumber int, headers []string, values []string) []models.ProcessingWarning {
	warnings := make([]models.ProcessingWarning, 0, 2)
	if len(values) < len(headers) {
		warnings = append(warnings, models.ProcessingWarning{
			Row:     rowNumber,
			Message: "В строке меньше значений, чем заголовков; недостающие ячейки заполнены пустыми значениями.",
		})
	}
	if len(values) > len(headers) && !tabular.IsEmptyRecord(values[len(headers):]) {
		warnings = append(warnings, models.ProcessingWarning{
			Row:     rowNumber,
			Message: "В строке есть лишние значения без заголовков; они не были сохранены.",
		})
	}

	return warnings
}
