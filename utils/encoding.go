// encoding.go – определение и декодирование кодировки CSV.

package utils

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

var ErrInvalidCSVEncoding = errors.New("Не удалось определить кодировку CSV")

// DecodeCSVContent декодирует CSV в UTF-8, возвращая (текст, кодировка, ошибка).
//
// Проверяет: бинарные данные -> BOM (UTF-8) -> valid UTF-8 -> Windows-1251.
func DecodeCSVContent(content []byte) (string, string, error) {
	if LooksBinary(content) {
		return "", "", ErrInvalidCSVEncoding
	}
	if bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
		return string(content[3:]), "UTF-8", nil
	}
	if utf8.Valid(content) {
		return string(content), "UTF-8", nil
	}

	decoded, err := charmap.Windows1251.NewDecoder().Bytes(content)
	if err != nil || !utf8.Valid(decoded) {
		return "", "", ErrInvalidCSVEncoding
	}

	return string(decoded), "Windows-1251", nil
}
