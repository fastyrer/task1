package services

import (
	"errors"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseCSVContentPreservesProcessingResult(t *testing.T) {
	content := []byte(
		"Отчёт по клиентам\n" +
			"Телефон;Имя;Скидка\n" +
			"89991234567;Анна;15\n" +
			"+420776654321;София;120\n",
	)

	data, err := parseCSVContent(content)
	if err != nil {
		t.Fatalf("parseCSVContent() error: %v", err)
	}

	if data.HeaderRow != 2 {
		t.Fatalf("unexpected header row: %d", data.HeaderRow)
	}
	if len(data.Rows) != 2 || len(data.RowNumbers) != 2 {
		t.Fatalf("unexpected rows: %#v, numbers: %#v", data.Rows, data.RowNumbers)
	}
	if data.RowNumbers[0] != 3 || data.RowNumbers[1] != 4 {
		t.Fatalf("unexpected source row numbers: %#v", data.RowNumbers)
	}
	if got := data.Rows[0]["Телефон"]; got != "+7 (999) 123-45-67" {
		t.Fatalf("unexpected normalized phone: %q", got)
	}
	if data.Stats.ValidRowCount != 1 || data.Stats.InvalidRowCount != 1 {
		t.Fatalf("unexpected stats: %#v", data.Stats)
	}
	if len(data.InvalidRows) != 1 || data.InvalidRows[0].Row != 4 {
		t.Fatalf("unexpected invalid rows: %#v", data.InvalidRows)
	}
}

func TestParseXLSXSelectedSheet(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()

	defaultSheet := workbook.GetSheetName(0)
	sheetName := "Клиенты"
	if _, err := workbook.NewSheet(sheetName); err != nil {
		t.Fatalf("create sheet: %v", err)
	}
	if err := workbook.SetCellValue(defaultSheet, "A1", "Пустой лист"); err != nil {
		t.Fatalf("set default sheet value: %v", err)
	}
	if err := workbook.SetSheetRow(sheetName, "A1", &[]any{"Телефон", "Имя"}); err != nil {
		t.Fatalf("set headers: %v", err)
	}
	if err := workbook.SetSheetRow(sheetName, "A2", &[]any{"89991234567", "Анна"}); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("serialize workbook: %v", err)
	}

	data, err := parseExcelContent(buffer.Bytes(), ParseOptions{SheetName: sheetName})
	if err != nil {
		t.Fatalf("parseExcelContent() error: %v", err)
	}
	if data.SheetName != sheetName || len(data.Rows) != 1 {
		t.Fatalf("unexpected sheet data: %#v", data)
	}
}

func TestDetectFileFormatRejectsMismatchedContent(t *testing.T) {
	_, err := detectFileFormat("clients.xlsx", []byte("Телефон,Имя\n"))
	if !errors.Is(err, ErrFileTypeMismatch) {
		t.Fatalf("expected file type mismatch, got %v", err)
	}
}
