// encoding.go – определение и декодирование кодировки CSV-файлов.
//
// CSV может быть в UTF-8 (с BOM или без) или в Windows-1251.
// Файл проверяет сигнатуру и конвертирует в UTF-8, возвращая
// название кодировки для отображения на фронте.
package utils

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// ErrInvalidEncoding возвращается, когда кодировку не удалось определить
// (бинарный файл, не UTF-8 и не Windows-1251).
var ErrInvalidEncoding = errors.New("Не удалось определить кодировку CSV.")

// DecodeCSVContent декодирует CSV-контент в UTF-8 и возвращает
// (строку, название кодировки, ошибку).
//
// Порядок проверки:
//  1. Если контент похож на бинарный (LooksBinary) – ошибка.
//  2. Если есть BOM (0xEF,0xBB,0xBF) – отрезает BOM, возвращает как UTF-8.
//  3. Если валидный UTF-8 – возвращает как есть.
//  4. Пробует декодировать как Windows-1251 (через golang.org/x/text).
//  5. Если не подошло ни то, ни другое – ErrInvalidEncoding.
//
// Возвращаемая строка:
//   - "UTF-8"        – файл был в UTF-8 или UTF-8 с BOM
//   - "Windows-1251" – файл был декодирован из Windows-1251
func DecodeCSVContent(content []byte) (string, string, error) {
	if LooksBinary(content) {
		return "", "", ErrInvalidEncoding
	}
	if bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
		return string(content[3:]), "UTF-8", nil
	}
	if utf8.Valid(content) {
		return string(content), "UTF-8", nil
	}

	decoded, err := charmap.Windows1251.NewDecoder().Bytes(content)
	if err != nil || !utf8.Valid(decoded) {
		return "", "", ErrInvalidEncoding
	}

	return string(decoded), "Windows-1251", nil
}
