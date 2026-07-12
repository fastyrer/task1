// Package services содержит парсеры файлов и преобразование их содержимого в models.FileData.
//
// excel_parser.go – отвечает за разбор Excel-файлов (XLS/XLSX): выбор листа, чтение строк,
// обработку объединенных ячеек и преобразование в общую структуру данных.

package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"

	"task1/models"
	"task1/utils"
)

// parseExcelContent – определяет формат Excel (XLSX или XLS) и делегирует разбор.
//
// parseExcelContent:
	// 1. Проверяет, является ли контент XLSX и вызывает parseXLSX
	// 2. Проверяет, является ли контент XLS и вызывает parseXLS
	// 3. Возвращает ErrInvalidExcel при неопознанном формате
func parseExcelContent(content []byte, options ParseOptions) (models.FileData, error) {

	// 1. Проверяет, является ли контент XLSX и вызывает parseXLSX
	if utils.IsXLSX(content) {
		return parseXLSX(bytes.NewReader(content), options)
	}

	// 2. Проверяет, является ли контент XLS и вызывает parseXLS
	if utils.IsXLS(content) {
		return parseXLS(bytes.NewReader(content), options)
	}

	// 3. Возвращает ErrInvalidExcel при неопознанном формате
	return models.FileData{}, ErrInvalidExcel
}

// parseXLSX – открывает XLSX-рабочую книгу и выбирает лист для парсинга.

// parseXLSX:
	// 1. Открывает книгу через excelize.OpenReader
	// 2. Получает список листов и проверяет наличие листов
	// 3. Если задан options.SheetName, находит индекс и парсит соответствующий лист
	// 4. Иначе пытает парсить листы по очереди, возвращая первый успешный результат
func parseXLSX(reader io.Reader, options ParseOptions) (models.FileData, error) {

	// 1. Открывает книгу через excelize.OpenReader
	workbook, err := excelize.OpenReader(reader)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	defer workbook.Close()

	// 2. Получает список листов и проверяет наличие листов
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	// 3. Если задан options.SheetName, находит индекс и парсит соответствующий лист
	if strings.TrimSpace(options.SheetName) != "" {
		sheetName := strings.TrimSpace(options.SheetName)
		if !utils.ContainsString(sheets, sheetName) {
			return models.FileData{}, fmt.Errorf("%w: %s.", ErrSheetNotFound, sheetName)
		}
		return parseXLSXSheet(workbook, sheetName, sheets)
	}

	var firstErr error

	// 4. Иначе пытает парсить листы по очереди, возвращая первый успешный результат
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

// parseXLSXSheet – читает строки из указанного листа XLSX, обрабатывает объединенные ячейки
// и преобразует результат в models.FileData.

// parseXLSXSheet:
	// 1. Получение всех строк листа через workbook.GetRows
	// 2. Заполнение объединенных ячеек через utils.FillMergedCells
	// 3. Преобразование строк в models.FileData через rowsToFileData
	// 4. Заполнение метаданных (SheetName, Sheets) и добавление предупреждений при мерджах
func parseXLSXSheet(workbook *excelize.File, sheetName string, sheets []string) (models.FileData, error) {

	// 1. Получение всех строк листа через workbook.GetRows
	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}

	// 2. Заполнение объединенных ячеек через utils.FillMergedCells
	rows, mergedCount, err := utils.FillMergedCells(workbook, sheetName, rows)
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}

	// 3. Преобразование строк в models.FileData через rowsToFileData
	data, err := rowsToFileData(rows)
	if err != nil {
		return models.FileData{}, err
	}

	// 4. Заполнение метаданных (SheetName, Sheets) и добавление предупреждений при мерджах
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

// parseXLS – открывает старый формат XLS и подготавливает имена листов для парсинга.

// parseXLS:
	// 1. Открывает книгу через xls.OpenReader
	// 2. Формирует список имен листов
	// 3. Если задан options.SheetName, находит индекс и парсит соответствующий лист
	// 4. Иначе пытает парсить листы по очереди, возвращая первый успешный результат
func parseXLS(reader io.ReadSeeker, options ParseOptions) (models.FileData, error) {

	// 1. Открывает книгу через xls.OpenReader
	workbook, err := xls.OpenReader(reader, "utf-8")
	if err != nil {
		return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidExcel, err)
	}
	if workbook.NumSheets() == 0 {
		return models.FileData{}, ErrEmptyFile
	}

	// 2. Формирует список имен листов
	sheetNames := make([]string, workbook.NumSheets())
	for index := 0; index < workbook.NumSheets(); index++ {
		sheetNames[index] = fmt.Sprintf("Лист %d", index+1)
	}

	// 3. Если задан options.SheetName, находит индекс и парсит соответствующий лист
	if strings.TrimSpace(options.SheetName) != "" {
		sheetIndex, ok := utils.SheetIndexByName(sheetNames, options.SheetName)
		if !ok {
			return models.FileData{}, fmt.Errorf("%w: %s.", ErrSheetNotFound, options.SheetName)
		}
		return parseXLSSheet(workbook.GetSheet(sheetIndex), sheetNames[sheetIndex], sheetNames)
	}

	var firstErr error
	// 4. Иначе пытает парсить листы по очереди, возвращая первый успешный результат
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

// parseXLSSheet – извлекает значения ячеек из xls.WorkSheet, очищает их и преобразует в models.FileData.

// parseXLSSheet:
	// 1. Проверка на пустой лист
	// 2. Итерация по всем строкам и столбцам, очистка значений через utils.CleanCell
	// 3. Сбор всех строк в [][]string и преобразование через rowsToFileData
	// 4. Установка метаданных (SheetName, Sheets)
func parseXLSSheet(sheet *xls.WorkSheet, sheetName string, sheets []string) (models.FileData, error) {

	// 1. Проверка на пустой лист
	if sheet == nil {
		return models.FileData{}, ErrEmptyFile
	}

	// 2. Итерация по всем строкам и столбцам, очистка значений через utils.CleanCell
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
			values[columnIndex] = utils.CleanCell(row.Col(columnIndex))
		}
		rows = append(rows, values)
	}

	// 3. Сбор всех строк в [][]string и преобразование через rowsToFileData
	data, err := rowsToFileData(rows)
	if err != nil {
		return models.FileData{}, err
	}
	
	// 4. Установка метаданных (SheetName, Sheets)
	data.SheetName = sheetName
	data.Sheets = sheets

	return data, nil
}
