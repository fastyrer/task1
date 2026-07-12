// excel.go – работа с Excel-файлами: объединённые ячейки и навигация по листам.
package utils

import (
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// FillMergedCells заполняет пустые ячейки объединённых диапазонов их значением.
//
// В Excel объединённые ячейки хранят значение только в первой (левой-верхней)
// ячейке, остальные ячейки диапазона пустые. При парсинге это приводит к
// потере данных. Функция находит все объединённые диапазоны на листе и
// копирует значение во все ячейки диапазона.
//
// Параметры:
//   - workbook – открытый XLSX-файл
//   - sheetName – имя листа
//   - rows – уже прочитанные строки листа (могут расширяться, если
//     объединённые ячейки выходят за текущие границы)
//
// Возвращает:
//   - обновлённый rows с заполненными ячейками
//   - количество обработанных объединений (для статистики)
//   - ошибку (если не удалось прочитать координаты ячейки)
//
// Пример: ячейки A1:B2 объединены со значением "Итого".
// До: rows = [["" ""] ["" ""]]
// После: rows = [["Итого" "Итого"] ["Итого" "Итого"]]
func FillMergedCells(workbook *excelize.File, sheetName string, rows [][]string) ([][]string, int, error) {
	mergeCells, err := workbook.GetMergeCells(sheetName)
	if err != nil {
		return rows, 0, err
	}

	for _, mergeCell := range mergeCells {
		value := CleanCell(mergeCell.GetCellValue())
		if value == "" {
			continue
		}

		startColumn, startRow, err := excelize.CellNameToCoordinates(mergeCell.GetStartAxis())
		if err != nil {
			return rows, 0, err
		}
		endColumn, endRow, err := excelize.CellNameToCoordinates(mergeCell.GetEndAxis())
		if err != nil {
			return rows, 0, err
		}

		for rowIndex := startRow - 1; rowIndex <= endRow-1; rowIndex++ {
			// Расширяем rows если объединение выходит за пределы
			for len(rows) <= rowIndex {
				rows = append(rows, nil)
			}
			for len(rows[rowIndex]) < endColumn {
				rows[rowIndex] = append(rows[rowIndex], "")
			}
			for columnIndex := startColumn - 1; columnIndex <= endColumn-1; columnIndex++ {
				if CleanCell(rows[rowIndex][columnIndex]) == "" {
					rows[rowIndex][columnIndex] = value
				}
			}
		}
	}

	return rows, len(mergeCells), nil
}

// SheetIndexByName ищет лист по имени или порядковому номеру (от 1).
//
// Параметры:
//   - sheetNames – список имён листов (из GetSheetList)
//   - target – имя листа ("Лист1", "Sheet2") или номер ("1", "2")
//
// Возвращает индекс в списке и true/false.
//
// Примеры:
//
//	SheetIndexByName(["Лист1", "Лист2"], "Лист2") → (1, true)
//	SheetIndexByName(["Sheet1", "Sheet2"], "2")   → (1, true)
//	SheetIndexByName(["A", "B"], "C")             → (0, false)
func SheetIndexByName(sheetNames []string, target string) (int, bool) {
	target = strings.TrimSpace(target)
	for index, name := range sheetNames {
		if name == target || strconv.Itoa(index+1) == target {
			return index, true
		}
	}

	return 0, false
}
