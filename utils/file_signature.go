package utils

import (
	"bytes"
	"net/http"
)

var (
	xlsxMagic = []byte("PK\x03\x04")
	xlsMagic  = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
)

func IsXLSX(content []byte) bool {
	return bytes.HasPrefix(content, xlsxMagic)
}

func IsXLS(content []byte) bool {
	return bytes.HasPrefix(content, xlsMagic)
}

// LooksBinary проверяет наличие бинарных данных в первых 512 байтах.
func LooksBinary(content []byte) bool {
	limit := len(content)
	if limit > 512 {
		limit = 512
	}
	if limit == 0 {
		return false
	}

	controlCount := 0
	for _, b := range content[:limit] {
		switch {
		case b == 0:
			return true
		case b == '\n' || b == '\r' || b == '\t':
			continue
		case b < 32:
			controlCount++
		}
	}

	return controlCount*4 > limit
}

// DetectedMIMEType возвращает MIME-тип по распознанному формату.
func DetectedMIMEType(format string, content []byte) string {
	switch format {
	case "csv":
		return "text/csv"
	case "xls":
		return "application/vnd.ms-excel"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return http.DetectContentType(content)
	}
}

// ExcelFormat определяет формат Excel по сигнатуре.
func ExcelFormat(content []byte) string {
	if IsXLSX(content) {
		return "xlsx"
	}
	return "xls"
}
