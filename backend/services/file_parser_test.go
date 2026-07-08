package services

import (
	"bytes"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

type readSeekCloser struct {
	*bytes.Reader
}

func (r readSeekCloser) Close() error {
	return nil
}

func multipartFile(content []byte) multipart.File {
	return readSeekCloser{Reader: bytes.NewReader(content)}
}

func TestParseCSV(t *testing.T) {
	data, err := ParseCSV(multipartFile([]byte("Телефон,Имя,Скидка\n+79990001122,Анна,15\n+79990003344,Иван\n")))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if len(data.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(data.Headers))
	}
	if len(data.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data.Rows))
	}
	if data.Rows[1]["Скидка"] != "" {
		t.Fatalf("expected missing value to be empty, got %q", data.Rows[1]["Скидка"])
	}
}

func TestParseCSVWithSemicolonDelimiter(t *testing.T) {
	data, err := ParseCSV(multipartFile([]byte("Телефон;Имя;Скидка\n+79990001122;Анна;15\n")))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if data.Rows[0]["Имя"] != "Анна" {
		t.Fatalf("expected semicolon-separated CSV to be parsed")
	}
}

func TestParseCSVWithPipeDelimiter(t *testing.T) {
	data, err := ParseCSV(multipartFile([]byte("Имя|Email\nАнна|ANNA@EXAMPLE.COM\n")))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if data.Rows[0]["Email"] != "anna@example.com" {
		t.Fatalf("expected email to be parsed and normalized, got %#v", data.Rows[0])
	}
}

func TestParseCSVWindows1251(t *testing.T) {
	content, err := charmap.Windows1251.NewEncoder().Bytes([]byte("Телефон;Имя\n+79990001122;Анна\n"))
	if err != nil {
		t.Fatalf("failed to encode test data: %v", err)
	}

	data, err := ParseCSV(multipartFile(content))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if data.Encoding != "Windows-1251" {
		t.Fatalf("expected Windows-1251 encoding, got %q", data.Encoding)
	}
	if data.Rows[0]["Имя"] != "Анна" {
		t.Fatalf("expected decoded name, got %#v", data.Rows[0])
	}
}

func TestParseCSVDetectsHeaderAfterTitleRow(t *testing.T) {
	data, err := ParseCSV(multipartFile([]byte("Отчет по клиентам\nТелефон,Имя,Скидка\n+79990001122,Анна,15\n")))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if data.HeaderRow != 2 {
		t.Fatalf("expected header row 2, got %d", data.HeaderRow)
	}
	if data.Stats.SkippedRowCount != 1 {
		t.Fatalf("expected one skipped row, got %#v", data.Stats)
	}
	if len(data.Warnings) == 0 || data.Warnings[0].Row != 1 {
		t.Fatalf("expected warning for skipped title row, got %#v", data.Warnings)
	}
}

func TestParseCSVEmptyFile(t *testing.T) {
	_, err := ParseCSV(multipartFile([]byte("  \n\t")))
	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}
}

func TestParseByFilenameRejectsMismatchedContent(t *testing.T) {
	_, err := ParseByFilename(multipartFile([]byte("PK\x03\x04fake xlsx")), "clients.csv")
	if !errors.Is(err, ErrFileTypeMismatch) {
		t.Fatalf("expected ErrFileTypeMismatch, got %v", err)
	}
}

func TestParseCSVValidationReport(t *testing.T) {
	content := []byte("Телефон,Email,Скидка,Дата\n123,bad,150,32.13.2024\n+79990001122,first@example.com,20,01.02.2024\n+7 999 000 33 44,dup@example.com,10,2024-01-02\n+79990003344,dup@example.com,10,2024-01-03\n")
	data, err := ParseCSV(multipartFile(content))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	if data.Stats.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %#v", data.Stats)
	}
	if data.Stats.InvalidRowCount != 2 {
		t.Fatalf("expected 2 invalid rows, got %#v", data.Stats)
	}
	if data.Stats.ValidRowCount != 2 {
		t.Fatalf("expected 2 valid rows, got %#v", data.Stats)
	}
	if len(data.InvalidRows) != 2 {
		t.Fatalf("expected invalid rows report, got %#v", data.InvalidRows)
	}
	if data.Rows[1]["Телефон"] != "+79990001122" {
		t.Fatalf("expected normalized phone, got %#v", data.Rows[1])
	}
}

func TestParseXLSX(t *testing.T) {
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	_ = workbook.SetCellValue(sheet, "A1", "Телефон")
	_ = workbook.SetCellValue(sheet, "B1", "Имя")
	_ = workbook.SetCellValue(sheet, "A2", "+79990001122")
	_ = workbook.SetCellValue(sheet, "B2", "Анна")

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	data, err := ParseExcel(multipartFile(buffer.Bytes()))
	if err != nil {
		t.Fatalf("ParseExcel returned error: %v", err)
	}

	if data.Rows[0]["Имя"] != "Анна" {
		t.Fatalf("expected xlsx row to be parsed")
	}
}

func TestParseXLSXSelectsFirstSheetWithData(t *testing.T) {
	workbook := excelize.NewFile()
	sheet, err := workbook.NewSheet("Clients")
	if err != nil {
		t.Fatalf("failed to create sheet: %v", err)
	}
	_ = workbook.SetCellValue("Clients", "A1", "Телефон")
	_ = workbook.SetCellValue("Clients", "B1", "Имя")
	_ = workbook.SetCellValue("Clients", "A2", "+79990001122")
	_ = workbook.SetCellValue("Clients", "B2", "Анна")
	workbook.SetActiveSheet(sheet)

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	data, err := ParseExcel(multipartFile(buffer.Bytes()))
	if err != nil {
		t.Fatalf("ParseExcel returned error: %v", err)
	}

	if data.SheetName != "Clients" {
		t.Fatalf("expected Clients sheet, got %q", data.SheetName)
	}
}
