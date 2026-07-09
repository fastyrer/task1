package utils

import "testing"

func TestFileSignatures(t *testing.T) {
	xlsx := []byte{'P', 'K', 0x03, 0x04, 0x00}
	xls := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

	if !IsXLSX(xlsx) || IsXLS(xlsx) {
		t.Fatal("unexpected XLSX signature result")
	}
	if !IsXLS(xls) || IsXLSX(xls) {
		t.Fatal("unexpected XLS signature result")
	}
	if ExcelFormat(xlsx) != "xlsx" || ExcelFormat(xls) != "xls" {
		t.Fatal("unexpected Excel format")
	}
}

func TestLooksBinary(t *testing.T) {
	if LooksBinary([]byte("Имя,Город\nАнна,Москва\n")) {
		t.Fatal("expected CSV text not to look binary")
	}
	if !LooksBinary([]byte{'a', 0x00, 'b'}) {
		t.Fatal("expected NUL byte to look binary")
	}
}

func TestDetectedMIMEType(t *testing.T) {
	tests := map[string]string{
		"csv":  "text/csv",
		"xls":  "application/vnd.ms-excel",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}

	for format, want := range tests {
		if got := DetectedMIMEType(format, nil); got != want {
			t.Fatalf("DetectedMIMEType(%q) = %q; want %q", format, got, want)
		}
	}
}
