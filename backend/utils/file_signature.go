package utils

import "net/http"

// IsXLSX проверяет ZIP-сигнатуру файла XLSX.
func IsXLSX(content []byte) bool {
	return len(content) >= 4 &&
		content[0] == 'P' &&
		content[1] == 'K' &&
		content[2] == 0x03 &&
		content[3] == 0x04
}

// IsXLS проверяет сигнатуру Compound File Binary файла XLS.
func IsXLS(content []byte) bool {
	return len(content) >= 8 &&
		content[0] == 0xD0 &&
		content[1] == 0xCF &&
		content[2] == 0x11 &&
		content[3] == 0xE0 &&
		content[4] == 0xA1 &&
		content[5] == 0xB1 &&
		content[6] == 0x1A &&
		content[7] == 0xE1
}

// LooksBinary определяет наличие бинарных данных в начале содержимого.
func LooksBinary(content []byte) bool {
	limit := len(content)
	if limit > 512 {
		limit = 512
	}
	if limit == 0 {
		return false
	}

	controlCount := 0
	for _, value := range content[:limit] {
		switch {
		case value == 0:
			return true
		case value == '\n' || value == '\r' || value == '\t':
			continue
		case value < 32:
			controlCount++
		}
	}

	return controlCount*4 > limit
}

// ExcelFormat возвращает формат Excel по сигнатуре содержимого.
func ExcelFormat(content []byte) string {
	if IsXLSX(content) {
		return "xlsx"
	}

	return "xls"
}

// DetectedMIMEType возвращает MIME-тип для распознанного формата.
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
