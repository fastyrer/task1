package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"

	"task1/internal/models"
	"task1/internal/tabular"
)

// parseExcelContent выбирает XLSX- или XLS-парсер по сигнатуре файла.
func parseExcelContent(content []byte, options ParseOptions) (models.FileData, error) {
	if tabular.IsXLSX(content) {
		return parseXLSX(bytes.NewReader(content), options)
	}

	if tabular.IsXLS(content) {
		return parseXLS(bytes.NewReader(content), options)
	}

	return models.FileData{}, ErrInvalidExcel
}

// parseXLSX открывает книгу и разбирает выбранный или первый подходящий лист.
func parseXLSX(reader io.Reader, options ParseOptions) (models.FileData, error) {
	workbook, err := excelize.OpenReader(reader)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w: %w", ErrInvalidExcel, err)
	}
	defer func() { _ = workbook.Close() }()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	if strings.TrimSpace(options.SheetName) != "" {
		sheetName := strings.TrimSpace(options.SheetName)
		if !tabular.ContainsString(sheets, sheetName) {
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

// parseXLSXSheet читает лист и разворачивает значения объединённых ячеек.
func parseXLSXSheet(workbook *excelize.File, sheetName string, sheets []string) (models.FileData, error) {
	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w: %w", ErrInvalidExcel, err)
	}

	rows, mergedCount, err := tabular.FillMergedCells(workbook, sheetName, rows)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w: %w", ErrInvalidExcel, err)
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
		RefreshStats(&data)
	}

	return data, nil
}

// parseXLS открывает книгу старого формата и разбирает выбранный или первый подходящий лист.
func parseXLS(reader io.ReadSeeker, options ParseOptions) (models.FileData, error) {
	workbook, err := xls.OpenReader(reader, "utf-8")
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w: %w", ErrInvalidExcel, err)
	}
	if workbook.NumSheets() == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	sheetNames := make([]string, workbook.NumSheets())
	for index := 0; index < workbook.NumSheets(); index++ {
		sheetNames[index] = fmt.Sprintf("Лист %d", index+1)
	}

	if strings.TrimSpace(options.SheetName) != "" {
		sheetIndex, ok := tabular.SheetIndexByName(sheetNames, options.SheetName)
		if !ok {
			return models.FileData{}, fmt.Errorf("%w: %s.", ErrSheetNotFound, options.SheetName)
		}
		return parseXLSSheet(workbook.GetSheet(sheetIndex), sheetNames[sheetIndex], sheetNames)
	}

	var firstErr error
	for index := 0; index < workbook.NumSheets(); index++ {
		data, err := parseXLSSheet(workbook.GetSheet(index), sheetNames[index], sheetNames)
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

// parseXLSSheet преобразует значения листа старого формата в FileData.
func parseXLSSheet(sheet *xls.WorkSheet, sheetName string, sheets []string) (models.FileData, error) {
	if sheet == nil {
		return models.FileData{}, ErrEmptyFile
	}

	rows := make([][]string, 0)
	for rowIndex := 0; rowIndex <= int(sheet.MaxRow); rowIndex++ {
		row := sheet.Row(rowIndex)
		if row == nil {
			rows = append(rows, nil)
			continue
		}

		lastColumn := row.LastCol()
		values := make([]string, lastColumn+1)
		for columnIndex := 0; columnIndex <= lastColumn; columnIndex++ {
			values[columnIndex] = tabular.CleanCell(row.Col(columnIndex))
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
