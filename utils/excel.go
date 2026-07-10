package utils

import (
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// FillMergedCells заполняет пустые ячейки объединённых диапазонов их значением.
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

// SheetIndexByName ищет лист по имени или порядковому номеру.
func SheetIndexByName(sheetNames []string, target string) (int, bool) {
	target = strings.TrimSpace(target)
	for index, name := range sheetNames {
		if name == target || strconv.Itoa(index+1) == target {
			return index, true
		}
	}

	return 0, false
}
