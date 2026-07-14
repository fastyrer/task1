package utils

import (
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// FillMergedCells копирует значение объединённых ячеек во все ячейки диапазона.
func FillMergedCells(workbook *excelize.File, sheetName string, rows [][]string) ([][]string, int, error) {
	// Получение объединенных ячеек
	mergeCells, err := workbook.GetMergeCells(sheetName)
	if err != nil {
		return rows, 0, err
	}

	// Получение значения из первой ячейки объединенной области
	for _, mc := range mergeCells {
		value := CleanCell(mc.GetCellValue())
		if value == "" {
			continue
		}

		startCol, startRow, err := excelize.CellNameToCoordinates(mc.GetStartAxis())
		if err != nil {
			return rows, 0, err
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(mc.GetEndAxis())
		if err != nil {
			return rows, 0, err
		}

		for r := startRow - 1; r <= endRow-1; r++ {
			for len(rows) <= r {
				rows = append(rows, nil)
			}
			for len(rows[r]) < endCol {
				rows[r] = append(rows[r], "")
			}
			for c := startCol - 1; c <= endCol-1; c++ {
				if CleanCell(rows[r][c]) == "" {
					rows[r][c] = value
				}
			}
		}
	}

	return rows, len(mergeCells), nil
}

// SheetIndexByName ищет лист по имени или порядковому номеру (от 1).
func SheetIndexByName(sheetNames []string, target string) (int, bool) {
	target = strings.TrimSpace(target)
	for i, name := range sheetNames {
		if name == target || strconv.Itoa(i+1) == target {
			return i, true
		}
	}
	return 0, false
}
