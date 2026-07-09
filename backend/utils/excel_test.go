package utils

import (
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestFillMergedCells(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()

	sheetName := workbook.GetSheetName(0)
	if err := workbook.SetCellValue(sheetName, "A1", "Группа"); err != nil {
		t.Fatalf("set merged value: %v", err)
	}
	if err := workbook.MergeCell(sheetName, "A1", "B2"); err != nil {
		t.Fatalf("merge cells: %v", err)
	}

	rows, count, err := FillMergedCells(workbook, sheetName, [][]string{{"Группа"}})
	if err != nil {
		t.Fatalf("FillMergedCells() error: %v", err)
	}
	want := [][]string{{"Группа", "Группа"}, {"Группа", "Группа"}}
	if count != 1 || !reflect.DeepEqual(rows, want) {
		t.Fatalf("unexpected merged rows: %#v, count %d", rows, count)
	}
}

func TestSheetIndexByName(t *testing.T) {
	sheets := []string{"Клиенты", "Архив"}

	if index, ok := SheetIndexByName(sheets, "Архив"); !ok || index != 1 {
		t.Fatalf("unexpected named sheet result: %d, %v", index, ok)
	}
	if index, ok := SheetIndexByName(sheets, "1"); !ok || index != 0 {
		t.Fatalf("unexpected numbered sheet result: %d, %v", index, ok)
	}
	if _, ok := SheetIndexByName(sheets, "Нет"); ok {
		t.Fatal("expected missing sheet")
	}
}
