package services

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"task1/backend/models"
)

const maxHeaderScanRows = 25

var (
	ErrUnsupportedFormat = errors.New("Неподдерживаемый формат файла. Загрузите CSV, XLS или XLSX.")
	ErrEmptyFile         = errors.New("Пустой файл.")
	ErrNoHeaders         = errors.New("В файле нет заголовков.")
	ErrNoDataRows        = errors.New("В файле нет строк с данными.")
	ErrInvalidCSV        = errors.New("Некорректная структура CSV.")
	ErrInvalidExcel      = errors.New("Некорректная структура XLS/XLSX.")
	ErrReadFile          = errors.New("Не удалось прочитать файл.")
	ErrFileTypeMismatch  = errors.New("Расширение файла не совпадает с содержимым.")
	ErrInvalidEncoding   = errors.New("Не удалось определить кодировку CSV.")
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

type columnKind string

const (
	columnGeneric  columnKind = ""
	columnPhone    columnKind = "phone"
	columnEmail    columnKind = "email"
	columnDiscount columnKind = "discount"
	columnDate     columnKind = "date"
)

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
	setFileFormat(&data, excelFormat(content), content)
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
		if isXLSX(content) || isXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "csv", nil
	case ".xls":
		if !isXLS(content) {
			return "", ErrFileTypeMismatch
		}
		return "xls", nil
	case ".xlsx":
		if !isXLSX(content) {
			return "", ErrFileTypeMismatch
		}
		return "xlsx", nil
	default:
		return "", ErrUnsupportedFormat
	}
}

func parseCSVContent(content []byte) (models.FileData, error) {
	text, encoding, err := decodeCSVContent(content)
	if err != nil {
		return models.FileData{}, err
	}
	if strings.TrimSpace(text) == "" {
		return models.FileData{}, ErrEmptyFile
	}

	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.Comma = detectCSVDelimiter(text)

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

	data, err := recordsToFileData(records)
	if err != nil {
		return models.FileData{}, err
	}
	data.Encoding = encoding
	if encoding != "UTF-8" {
		data.Warnings = append(data.Warnings, models.ProcessingWarning{
			Message: fmt.Sprintf("CSV декодирован из %s.", encoding),
		})
		refreshStats(&data)
	}

	return data, nil
}

func decodeCSVContent(content []byte) (string, string, error) {
	if looksBinary(content) {
		return "", "", ErrInvalidEncoding
	}
	if bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
		return string(content[3:]), "UTF-8", nil
	}
	if utf8.Valid(content) {
		return string(content), "UTF-8", nil
	}

	decoded, err := charmap.Windows1251.NewDecoder().Bytes(content)
	if err != nil || !utf8.Valid(decoded) {
		return "", "", ErrInvalidEncoding
	}

	return string(decoded), "Windows-1251", nil
}

func parseExcelContent(content []byte, options ParseOptions) (models.FileData, error) {
	if isXLSX(content) {
		return parseXLSX(bytes.NewReader(content), options)
	}
	if isXLS(content) {
		return parseXLS(bytes.NewReader(content), options)
	}

	return models.FileData{}, ErrInvalidExcel
}

func setFileFormat(data *models.FileData, format string, content []byte) {
	data.Format = format
	data.DetectedMIMEType = detectedMIMEType(format, content)
}

func excelFormat(content []byte) string {
	if isXLSX(content) {
		return "xlsx"
	}

	return "xls"
}

func detectedMIMEType(format string, content []byte) string {
	switch format {
	case "csv":
		return "text/csv"
	case "xls":
		return "application/vnd.ms-excel"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return http.DetectContentType(content)
	}
}

func parseXLSX(reader io.Reader, options ParseOptions) (models.FileData, error) {
	workbook, err := excelize.OpenReader(reader)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	defer workbook.Close()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	if strings.TrimSpace(options.SheetName) != "" {
		sheetName := strings.TrimSpace(options.SheetName)
		if !containsString(sheets, sheetName) {
			return models.FileData{}, fmt.Errorf("%w: %s.", ErrSheetNotFound, sheetName)
		}
		return parseXLSXSheet(workbook, sheetName, sheets)
	}

	var firstErr error
	for _, sheetName := range sheets {
		data, err := parseXLSXSheet(workbook, sheetName, sheets)
		if err == nil {
			return data, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return models.FileData{}, firstErr
	}

	return models.FileData{}, ErrEmptyFile
}

func parseXLSXSheet(workbook *excelize.File, sheetName string, sheets []string) (models.FileData, error) {
	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}

	rows, mergedCount, err := fillMergedCells(workbook, sheetName, rows)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}

	data, err := rowsToFileData(rows)
	if err != nil {
		return models.FileData{}, err
	}
	data.SheetName = sheetName
	data.Sheets = sheets
	if mergedCount > 0 {
		data.Warnings = append(data.Warnings, models.ProcessingWarning{
			Message: fmt.Sprintf("На листе обработаны объединенные ячейки: %d.", mergedCount),
		})
		refreshStats(&data)
	}

	return data, nil
}

func fillMergedCells(workbook *excelize.File, sheetName string, rows [][]string) ([][]string, int, error) {
	mergeCells, err := workbook.GetMergeCells(sheetName)
	if err != nil {
		return rows, 0, err
	}

	for _, mergeCell := range mergeCells {
		startAxis := mergeCell.GetStartAxis()
		endAxis := mergeCell.GetEndAxis()
		value := cleanCell(mergeCell.GetCellValue())
		if value == "" {
			continue
		}

		startCol, startRow, err := excelize.CellNameToCoordinates(startAxis)
		if err != nil {
			return rows, 0, err
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(endAxis)
		if err != nil {
			return rows, 0, err
		}

		for rowIndex := startRow - 1; rowIndex <= endRow-1; rowIndex++ {
			for len(rows) <= rowIndex {
				rows = append(rows, nil)
			}
			for len(rows[rowIndex]) < endCol {
				rows[rowIndex] = append(rows[rowIndex], "")
			}
			for colIndex := startCol - 1; colIndex <= endCol-1; colIndex++ {
				if cleanCell(rows[rowIndex][colIndex]) == "" {
					rows[rowIndex][colIndex] = value
				}
			}
		}
	}

	return rows, len(mergeCells), nil
}

func parseXLS(reader io.ReadSeeker, options ParseOptions) (models.FileData, error) {
	workbook, err := xls.OpenReader(reader, "utf-8")
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	if workbook.NumSheets() == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	sheetNames := make([]string, workbook.NumSheets())
	for i := 0; i < workbook.NumSheets(); i++ {
		sheetNames[i] = fmt.Sprintf("Лист %d", i+1)
	}

	if strings.TrimSpace(options.SheetName) != "" {
		sheetIndex, ok := sheetIndexByName(sheetNames, options.SheetName)
		if !ok {
			return models.FileData{}, fmt.Errorf("%w: %s.", ErrSheetNotFound, options.SheetName)
		}
		return parseXLSSheet(workbook.GetSheet(sheetIndex), sheetNames[sheetIndex], sheetNames)
	}

	var firstErr error
	for i := 0; i < workbook.NumSheets(); i++ {
		data, err := parseXLSSheet(workbook.GetSheet(i), sheetNames[i], sheetNames)
		if err == nil {
			return data, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return models.FileData{}, firstErr
	}

	return models.FileData{}, ErrEmptyFile
}

func parseXLSSheet(sheet *xls.WorkSheet, sheetName string, sheets []string) (models.FileData, error) {
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

	data, err := rowsToFileData(rows)
	if err != nil {
		return models.FileData{}, err
	}
	data.SheetName = sheetName
	data.Sheets = sheets

	return data, nil
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
		values := trimTrailingEmptyCells(record.Values)
		if isEmptyRecord(values) {
			emptyRowCount++
			continue
		}

		rowWarnings := rowShapeWarnings(record.Number, headers, values)
		warnings = append(warnings, rowWarnings...)
		dataRows = append(dataRows, parsedDataRow{
			Number: record.Number,
			Values: recordToMap(headers, values),
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

func detectHeaderRecord(records []parsedRecord) (int, []string, []models.ProcessingWarning, error) {
	limit := len(records)
	if limit > maxHeaderScanRows {
		limit = maxHeaderScanRows
	}

	bestIndex := -1
	bestScore := -1 << 30
	var bestHeaders []string
	for i := 0; i < limit; i++ {
		values := trimTrailingEmptyCells(records[i].Values)
		if isEmptyRecord(values) {
			continue
		}

		headers, err := normalizeHeaders(values)
		if err != nil {
			continue
		}

		score := scoreHeaderCandidate(headers, records, i)
		if score > bestScore {
			bestIndex = i
			bestScore = score
			bestHeaders = headers
		}
	}

	if bestIndex == -1 {
		for i, record := range records {
			if isEmptyRecord(record.Values) {
				continue
			}
			headers, err := normalizeHeaders(record.Values)
			if err != nil {
				return 0, nil, nil, err
			}
			return i, headers, skippedBeforeHeaderWarnings(records, i), nil
		}
		return 0, nil, nil, ErrNoHeaders
	}

	return bestIndex, bestHeaders, skippedBeforeHeaderWarnings(records, bestIndex), nil
}

func scoreHeaderCandidate(headers []string, records []parsedRecord, index int) int {
	score := len(headers) * 3
	for _, header := range headers {
		if isCommonHeader(header) {
			score += 6
		}
		if isNumberLike(header) {
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
		if !isEmptyRecord(record.Values) {
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
		if isEmptyRecord(record.Values) {
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
	if len(values) > len(headers) && !isEmptyRecord(values[len(headers):]) {
		warnings = append(warnings, models.ProcessingWarning{
			Row:     rowNumber,
			Message: "В строке есть лишние значения без заголовков; они не были сохранены.",
		})
	}

	return warnings
}

func normalizeHeaders(record []string) ([]string, error) {
	record = trimTrailingEmptyCells(record)
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

		key := headerKey(header)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("В файле есть повторяющийся заголовок: %s.", header)
		}

		seen[key] = struct{}{}
		headers[i] = header
	}

	return headers, nil
}

func validateRows(headers []string, rows []parsedDataRow) ([]models.InvalidRow, []models.ProcessingWarning) {
	kinds := make(map[string]columnKind, len(headers))
	for _, header := range headers {
		kinds[header] = classifyHeader(header)
	}

	seenValues := make(map[string]map[string]int)
	invalidRows := make([]models.InvalidRow, 0)
	warnings := make([]models.ProcessingWarning, 0)
	for _, row := range rows {
		rowErrors := make([]models.ProcessingWarning, 0)
		for _, header := range headers {
			value := cleanCell(row.Values[header])
			row.Values[header] = value
			if value == "" {
				continue
			}

			switch kinds[header] {
			case columnPhone:
				normalized, ok := normalizePhone(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный телефон."))
					continue
				}
				row.Values[header] = normalized
				rowErrors = append(rowErrors, duplicateWarning(seenValues, header, normalized, row.Number)...)
			case columnEmail:
				normalized, ok := normalizeEmail(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Некорректный email."))
					continue
				}
				row.Values[header] = normalized
				rowErrors = append(rowErrors, duplicateWarning(seenValues, header, normalized, row.Number)...)
			case columnDiscount:
				normalized, ok := normalizePercent(value)
				if !ok {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Скидка должна быть числом от 0 до 100."))
					continue
				}
				row.Values[header] = normalized
			case columnDate:
				if !isSupportedDate(value) {
					rowErrors = append(rowErrors, fieldWarning(row.Number, header, "Дата должна быть в распознаваемом формате."))
				}
			}
		}

		if len(rowErrors) > 0 {
			warnings = append(warnings, rowErrors...)
			invalidRows = append(invalidRows, models.InvalidRow{
				Row:    row.Number,
				Values: cloneRow(row.Values),
				Errors: rowErrors,
			})
		}
	}

	return invalidRows, warnings
}

func fieldWarning(row int, column string, message string) models.ProcessingWarning {
	return models.ProcessingWarning{
		Row:     row,
		Column:  column,
		Message: message,
	}
}

func duplicateWarning(seenValues map[string]map[string]int, column string, value string, row int) []models.ProcessingWarning {
	if seenValues[column] == nil {
		seenValues[column] = make(map[string]int)
	}

	if firstRow, ok := seenValues[column][value]; ok {
		return []models.ProcessingWarning{fieldWarning(row, column, fmt.Sprintf("Дубликат значения; впервые встречено в строке %d.", firstRow))}
	}

	seenValues[column][value] = row
	return nil
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

func detectCSVDelimiter(content string) rune {
	candidates := []rune{',', ';', '\t', '|'}
	lines := sampleCSVLines(content, 10)
	if len(lines) == 0 {
		return ','
	}

	bestDelimiter := ','
	bestScore := -1 << 30
	for _, delimiter := range candidates {
		score := delimiterScore(lines, delimiter)
		if score > bestScore {
			bestDelimiter = delimiter
			bestScore = score
		}
	}

	return bestDelimiter
}

func sampleCSVLines(content string, limit int) []string {
	lines := make([]string, 0, limit)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == limit {
			break
		}
	}

	return lines
}

func delimiterScore(lines []string, delimiter rune) int {
	counts := make(map[int]int)
	totalFields := 0
	for _, line := range lines {
		fieldCount := strings.Count(line, string(delimiter)) + 1
		counts[fieldCount]++
		totalFields += fieldCount
	}

	if totalFields == len(lines) {
		return -1
	}

	bestConsistency := 0
	for _, count := range counts {
		if count > bestConsistency {
			bestConsistency = count
		}
	}

	return totalFields + bestConsistency*5
}

func cleanHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}

func cleanCell(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.TrimSpace(value)
}

func trimTrailingEmptyCells(record []string) []string {
	last := len(record)
	for last > 0 && cleanCell(record[last-1]) == "" {
		last--
	}

	return record[:last]
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

func isXLS(content []byte) bool {
	return len(content) >= 8 &&
		content[0] == 0xD0 &&
		content[1] == 0xCF &&
		content[2] == 0x11 &&
		content[3] == 0xE0 &&
		content[4] == 0xA1 &&
		content[5] == 0xB1 &&
		content[6] == 0x1A &&
		content[7] == 0xE1
}

func looksBinary(content []byte) bool {
	limit := len(content)
	if limit > 512 {
		limit = 512
	}
	if limit == 0 {
		return false
	}

	controlCount := 0
	for _, value := range content[:limit] {
		switch {
		case value == 0:
			return true
		case value == '\n' || value == '\r' || value == '\t':
			continue
		case value < 32:
			controlCount++
		}
	}

	return controlCount*4 > limit
}

func classifyHeader(header string) columnKind {
	key := headerKey(header)
	switch {
	case strings.Contains(key, "телефон"), key == "phone", strings.Contains(key, "mobile"):
		return columnPhone
	case key == "email", key == "e-mail", strings.Contains(key, "почта"), strings.Contains(key, "mail"):
		return columnEmail
	case strings.Contains(key, "скидка"), strings.Contains(key, "discount"), strings.Contains(key, "процент"):
		return columnDiscount
	case strings.Contains(key, "дата"), key == "date", strings.HasSuffix(key, " date"):
		return columnDate
	default:
		return columnGeneric
	}
}

func isCommonHeader(header string) bool {
	key := headerKey(header)
	if classifyHeader(header) != columnGeneric {
		return true
	}

	switch key {
	case "имя", "фио", "name", "first name", "last name", "клиент", "client", "город", "city":
		return true
	default:
		return false
	}
}

func headerKey(header string) string {
	key := strings.ToLower(cleanHeader(header))
	key = strings.ReplaceAll(key, "ё", "е")
	return key
}

func normalizePhone(value string) (string, bool) {

	var digits strings.Builder
	for _, symbol := range value {
		if symbol >= '0' && symbol <= '9' {
			digits.WriteRune(symbol)
		}
	}

	number := digits.String()
	var normalized string

	switch {
	case len(number) == 10:
		normalized = "+7" + number
	case len(number) == 11 && strings.HasPrefix(number, "8"):
		normalized = "+7" + number[1:]
	case len(number) == 11 && strings.HasPrefix(number, "7"):
		normalized = "+" + number
	default:
		return "", false
	}

	firstCodeDigit := normalized[2]
	if firstCodeDigit != '3' && firstCodeDigit != '4' && firstCodeDigit != '9' {
		return "", false
	}

	// +7XXXXXXXXXX --> +7 (XXX) XXX-XX-XX
	return fmt.Sprintf( "+7 (%s) %s-%s-%s",
		normalized[2:5],
		normalized[5:8],
		normalized[8:10],
		normalized[10:12],
	), true
}

func normalizeEmail(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}

	address, err := mail.ParseAddress(value)
	if err != nil || !strings.Contains(address.Address, "@") {
		return "", false
	}

	return strings.ToLower(address.Address), true
}

func normalizePercent(value string) (string, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || number > 100 {
		return "", false
	}

	return strconv.FormatFloat(number, 'f', -1, 64), true
}

func isSupportedDate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}

	layouts := []string{
		"2006-01-02",
		"02.01.2006",
		"2.1.2006",
		"02/01/2006",
		"2/1/2006",
		"01/02/2006",
		"1/2/2006",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}

	return false
}

func isNumberLike(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func cloneRow(row map[string]string) map[string]string {
	clone := make(map[string]string, len(row))
	for key, value := range row {
		clone[key] = value
	}

	return clone
}

func refreshStats(data *models.FileData) {
	data.Stats.ColumnCount = len(data.Headers)
	data.Stats.RowCount = len(data.Rows)
	data.Stats.InvalidRowCount = len(data.InvalidRows)
	data.Stats.ValidRowCount = data.Stats.RowCount - data.Stats.InvalidRowCount
	if data.Stats.ValidRowCount < 0 {
		data.Stats.ValidRowCount = 0
	}
	data.Stats.WarningCount = len(data.Warnings)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func DetectPhoneColumn(headers []string) string {
	for _, h := range headers {
		if classifyHeader(h) == columnPhone {
			return h
		}
	}
	return ""
}

func sheetIndexByName(sheetNames []string, target string) (int, bool) {
	target = strings.TrimSpace(target)
	for index, name := range sheetNames {
		if name == target || strconv.Itoa(index+1) == target {
			return index, true
		}
	}

	return 0, false
}
