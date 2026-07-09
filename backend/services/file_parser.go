package services

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"task1/backend/models"
	"task1/backend/utils"
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
	ErrInvalidEncoding   = utils.ErrInvalidEncoding
	ErrSheetNotFound     = errors.New("Лист Excel не найден.")
)

type ParseOptions struct {
	SheetName string
}

type parsedRecord struct {
	Number int
	Values []string
}

type parsedDataRow struct {
	Number int
	Values map[string]string
}

func ParseByFilename(file multipart.File, filename string) (models.FileData, error) {
	return ParseByFilenameWithOptions(file, filename, ParseOptions{})
}

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

func ParseCSV(file multipart.File) (models.FileData, error) {
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	data, err := parseCSVContent(content)
	if err != nil {
		return models.FileData{}, err
	}
	setFileFormat(&data, "csv", content)
	return data, nil
}

func ParseExcel(file multipart.File) (models.FileData, error) {
	content, err := readFileContent(file)
	if err != nil {
		return models.FileData{}, err
	}

	data, err := parseExcelContent(content, ParseOptions{})
	if err != nil {
		return models.FileData{}, err
	}
	setFileFormat(&data, utils.ExcelFormat(content), content)
	return data, nil
}

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

func detectFileFormat(filename string, content []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		if utils.IsXLSX(content) || utils.IsXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "csv", nil
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
	default:
		return "", ErrUnsupportedFormat
	}
}

func setFileFormat(data *models.FileData, format string, content []byte) {
	data.Format = format
	data.DetectedMIMEType = utils.DetectedMIMEType(format, content)
}

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

func recordsToFileData(records []parsedRecord) (models.FileData, error) {
	if len(records) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	headerIndex, headers, warnings, err := detectHeaderRecord(records)
	if err != nil {
		return models.FileData{}, err
	}

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
	refreshStats(&data)

	return data, nil
}
