// file_signature.go – определение формата файла по magic-байтам (сигнатуре).
//
// Расширение файла можно подделать (.csv → .xlsx). Проверка по сигнатуре
// (первые байты содержимого) — единственный надёжный способ отличить
// XLSX (ZIP-архив) от XLS (Compound Document) от CSV (текст).
package utils

import "net/http"

// IsXLSX проверяет ZIP-сигнатуру файла XLSX.
//
// XLSX — это ZIP-архив, первые 4 байта: PK\x03\x04.
// Совпадает с сигнатурой любого ZIP-файла, но для XLSX достаточно.
func IsXLSX(content []byte) bool {
	return len(content) >= 4 &&
		content[0] == 'P' &&
		content[1] == 'K' &&
		content[2] == 0x03 &&
		content[3] == 0x04
}

// IsXLS проверяет сигнатуру Compound File Binary для XLS.
//
// Старый формат XLS (до 2007) использует OLE2 Compound Document.
// Первые 8 байт — фиксированная сигнатура D0 CF 11 E0 A1 B1 1A E1.
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

// LooksBinary определяет наличие бинарных данных в первых 512 байтах.
//
// Алгоритм:
//   - нулевой байт (0x00) — сразу true (текстовый файл не содержит нулей)
//   - управляющие символы (< 32, кроме \n \r \t) — считаются
//   - если таких символов больше 25% — содержимое считается бинарным
//
// Используется в DecodeCSVContent перед попыткой декодирования кодировки:
// бинарный XLS/XLSX не нужно пытаться декодировать как CSV.
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

// ExcelFormat возвращает формат Excel ("xlsx" или "xls") по сигнатуре.
func ExcelFormat(content []byte) string {
	if IsXLSX(content) {
		return "xlsx"
	}

	return "xls"
}

// DetectedMIMEType возвращает MIME-тип для распознанного формата.
//
// Для CSV/XLS/XLSX возвращает известный MIME-тип.
// Для неизвестного формата — через http.DetectContentType.
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
