package services

import (
	"bytes"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/xuri/excelize/v2"
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

func TestParseCSVEmptyFile(t *testing.T) {
	_, err := ParseCSV(multipartFile([]byte("  \n\t")))
	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
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
