// normalizers.go – приведение данных к единому формату.

package utils

import (
	"net/mail"
	"strconv"
	"strings"
)

const maxPhoneDigits = 11

func NormalizePhone(value string) (string, bool) {
	var digits strings.Builder
	digits.Grow(maxPhoneDigits)
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	number := digits.String()

	var normalized string
	switch {
	case len(number) == 10:
		normalized = "+7" + number
	case len(number) == 11 && number[0] == '8':
		normalized = "+7" + number[1:]
	case len(number) == 11 && number[0] == '7':
		normalized = "+" + number
	default:
		return "", false
	}

	firstCodeDigit := normalized[2]
	if firstCodeDigit != '3' && firstCodeDigit != '4' && firstCodeDigit != '9' {
		return "", false
	}

	return normalized, true
}

func NormalizeEmail(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}

	addr, err := mail.ParseAddress(value)
	if err != nil || !strings.Contains(addr.Address, "@") {
		return "", false
	}

	return strings.ToLower(addr.Address), true
}

func NormalizePercent(value string) (string, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", ".")
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || n < 0 || n > 100 {
		return "", false
	}
	return strconv.FormatFloat(n, 'f', -1, 64), true
}
