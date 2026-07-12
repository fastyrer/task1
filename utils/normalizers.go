// normalizers.go – приведение данных к единому формату (нормализация).
//
// Поддерживается нормализация телефонов (РФ, +7), email (нижний регистр),
// процентов (0–100). А также проверка дат и чисел.
package utils

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// NormalizePhone приводит российский номер к формату +7 (XXX) XXX-XX-XX.
//
// Принимает:
//   - 10 цифр (9161234567 → +7 (916) 123-45-67)
//   - 11 цифр с 8 (89161234567 → +7 (916) 123-45-67)
//   - 11 цифр с 7 (79161234567 → +7 (916) 123-45-67)
//
// Ограничения:
//   - Только РФ (код страны 7). Международные номера (10–15 цифр с +) не поддерживаются.
//   - Первая цифра кода оператора: 3, 4 или 9 (соответствует российским DEF-кодам).
//     Номера вида +7 (200) XXX-XX-XX отклоняются.
//
// Возвращает false, если номер не подходит под формат РФ или содержит
// неподдерживаемый код оператора.
func NormalizePhone(value string) (string, bool) {
	var digits strings.Builder
	for _, symbol := range value {
		if symbol >= '0' && symbol <= '9' {
			digits.WriteRune(symbol)
		}
	}

	number := digits.String()
	var normalized string

	switch {
	case len(number) == 10:
		normalized = "+7" + number
	case len(number) == 11 && strings.HasPrefix(number, "8"):
		normalized = "+7" + number[1:]
	case len(number) == 11 && strings.HasPrefix(number, "7"):
		normalized = "+" + number
	default:
		return "", false
	}

	// Проверка кода оператора: первая цифра после +7
	firstCodeDigit := normalized[2]
	if firstCodeDigit != '3' && firstCodeDigit != '4' && firstCodeDigit != '9' {
		return "", false
	}

	return fmt.Sprintf(
		"+7 (%s) %s-%s-%s",
		normalized[2:5],
		normalized[5:8],
		normalized[8:10],
		normalized[10:12],
	), true
}

// NormalizeEmail проверяет корректность email и приводит к нижнему регистру.
//
// Требования:
//   - Без пробелов и табуляций
//   - Проходит mail.ParseAddress
//   - Содержит @
//
// Возвращает false для "name @domain.com" (пробел),
// для пустой строки, для строки без @.
func NormalizeEmail(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}

	address, err := mail.ParseAddress(value)
	if err != nil || !strings.Contains(address.Address, "@") {
		return "", false
	}

	return strings.ToLower(address.Address), true
}

// NormalizePercent нормализует процентное значение от 0 до 100.
//
// Принимает:
//   - "15" → "15"
//   - "15%" → "15" (отрезает %)
//   - "15,5" → "15.5" (заменяет запятую на точку)
//   - "-1" или "101" → false (выход за диапазон)
//   - "abc" → false (не число)
func NormalizePercent(value string) (string, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || number > 100 {
		return "", false
	}

	return strconv.FormatFloat(number, 'f', -1, 64), true
}

// IsSupportedDate проверяет, соответствует ли строка одному из поддерживаемых
// форматов даты.
//
// Поддерживаются:
//
//	2006-01-02      (ISO)
//	02.01.2006      (ДД.ММ.ГГГГ)
//	2.1.2006        (Д.М.ГГГГ)
//	02/01/2006      (ДД/ММ/ГГГГ)
//	2/1/2006        (Д/М/ГГГГ)
//	01/02/2006      (ММ/ДД/ГГГГ — американский)
//	1/2/2006        (М/Д/ГГГГ)
//	2006/01/02      (ГГГГ/ММ/ДД)
//
// Пустая строка считается валидной датой (значение не указано).
// Возвращает только true/false, саму дату не нормализует.
func IsSupportedDate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}

	layouts := []string{
		"2006-01-02",
		"02.01.2006",
		"2.1.2006",
		"02/01/2006",
		"2/1/2006",
		"01/02/2006",
		"1/2/2006",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}

	return false
}

// IsNumberLike проверяет, можно ли интерпретировать строку как число
// (целое или с плавающей точкой).
//
// Полезна для скоринга заголовков: если значение похоже на число —
// это вероятно данные, а не заголовок (штраф в scoreHeaderCandidate).
//
// Примеры:
//
//	IsNumberLike("123") → true
//	IsNumberLike("12.5") → true
//	IsNumberLike("12,5") → true (замена запятой)
//	IsNumberLike("ABC") → false
//	IsNumberLike("") → false
func IsNumberLike(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	if value == "" {
		return false
	}

	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}
