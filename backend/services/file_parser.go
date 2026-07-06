package services

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"

	"task1/backend/models"
)

var (
	ErrUnsupportedFormat = errors.New("Неподдерживаемый формат файла. Загрузите CSV, XLS или XLSX.")
	ErrEmptyFile         = errors.New("Пустой файл.")
	ErrNoHeaders         = errors.New("В файле нет заголовков.")
	ErrNoDataRows        = errors.New("В файле нет строк с данными.")
	ErrInvalidCSV        = errors.New("Некорректная структура CSV.")
	ErrInvalidExcel      = errors.New("Некорректная структура XLS/XLSX.")
	ErrReadFile          = errors.New("Не удалось прочитать файл.")
)

func ParseByFilename(file multipart.File, filename string) (models.FileData, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		return ParseCSV(file)
	case ".xls", ".xlsx":
		return ParseExcel(file)
	default:
		return models.FileData{}, ErrUnsupportedFormat
	}
}

func ParseCSV(file multipart.File) (models.FileData, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return models.FileData{}, ErrReadFile
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.Comma = detectCSVDelimiter(content)

	headerRecord, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return models.FileData{}, ErrEmptyFile
	}
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidCSV, err)
	}

	headers, err := normalizeHeaders(headerRecord)
	if err != nil {
		return models.FileData{}, err
	}

	rows := make([]map[string]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidCSV, err)
		}
		if isEmptyRecord(record) {
			continue
		}

		rows = append(rows, recordToMap(headers, record))
	}

	if len(rows) == 0 {
		return models.FileData{}, ErrNoDataRows
	}

	return models.FileData{
		Headers: headers,
		Rows:    rows,
	}, nil
}

func ParseExcel(file multipart.File) (models.FileData, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return models.FileData{}, ErrReadFile
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	if isXLSX(content) {
		return parseXLSX(bytes.NewReader(content))
	}

	return parseXLS(bytes.NewReader(content))
}

func parseXLSX(reader io.Reader) (models.FileData, error) {
	workbook, err := excelize.OpenReader(reader)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	defer workbook.Close()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}

	return rowsToFileData(rows)
}

func parseXLS(reader io.ReadSeeker) (models.FileData, error) {
	workbook, err := xls.OpenReader(reader, "utf-8")
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	if workbook.NumSheets() == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	sheet := workbook.GetSheet(0)
	if sheet == nil {
		return models.FileData{}, ErrEmptyFile
	}

	rows := make([][]string, 0)
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			rows = append(rows, nil)
			continue
		}

		lastCol := row.LastCol()
		values := make([]string, lastCol+1)
		for col := 0; col <= lastCol; col++ {
			values[col] = cleanCell(row.Col(col))
		}
		rows = append(rows, values)
	}

	return rowsToFileData(rows)
}

func rowsToFileData(rows [][]string) (models.FileData, error) {
	if len(rows) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	headers, err := normalizeHeaders(rows[0])
	if err != nil {
		return models.FileData{}, err
	}

	dataRows := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if isEmptyRecord(row) {
			continue
		}
		dataRows = append(dataRows, recordToMap(headers, row))
	}

	if len(dataRows) == 0 {
		return models.FileData{}, ErrNoDataRows
	}

	return models.FileData{
		Headers: headers,
		Rows:    dataRows,
	}, nil
}

func normalizeHeaders(record []string) ([]string, error) {
	if len(record) == 0 || isEmptyRecord(record) {
		return nil, ErrNoHeaders
	}

	headers := make([]string, len(record))
	seen := make(map[string]struct{}, len(record))
	for i, header := range record {
		header = cleanHeader(header)
		if header == "" {
			return nil, errors.New("В файле есть пустые заголовки.")
		}
		if _, ok := seen[header]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}

		seen[header] = struct{}{}
		headers[i] = header
	}

	return headers, nil
}

func recordToMap(headers []string, record []string) map[string]string {
	row := make(map[string]string, len(headers))
	for i, header := range headers {
		value := ""
		if i < len(record) {
			value = cleanCell(record[i])
		}
		row[header] = value
	}

	return row
}

func detectCSVDelimiter(content []byte) rune {
	firstLine := string(content)
	if idx := strings.IndexAny(firstLine, "\r\n"); idx >= 0 {
		firstLine = firstLine[:idx]
	}

	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		return ';'
	}

	return ','
}

func cleanHeader(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
}

func cleanCell(value string) string {
	return strings.TrimSpace(value)
}

func isEmptyRecord(record []string) bool {
	for _, value := range record {
		if cleanCell(value) != "" {
			return false
		}
	}

	return true
}

func isXLSX(content []byte) bool {
	return len(content) >= 4 &&
		content[0] == 'P' &&
		content[1] == 'K' &&
		content[2] == 0x03 &&
		content[3] == 0x04
}
