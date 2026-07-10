package utils

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// NormalizePhone приводит поддерживаемый российский номер к единому формату.
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

// NormalizeEmail проверяет адрес и приводит его к нижнему регистру.
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
func NormalizePercent(value string) (string, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || number > 100 {
		return "", false
	}

	return strconv.FormatFloat(number, 'f', -1, 64), true
}

// IsSupportedDate проверяет дату по поддерживаемым форматам.
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

// IsNumberLike проверяет, можно ли интерпретировать значение как число.
func IsNumberLike(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	if value == "" {
		return false
	}

	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}
