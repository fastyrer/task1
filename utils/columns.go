// columns.go – классификация колонок по точному совпадению заголовка.
//
// Определяет тип колонки (телефон, email, скидка, дата) по тексту заголовка
// через точное сравнение. Используется при валидации строк (row_validator)
// и в обработчиках для поиска колонки телефона.
package utils

// ColumnKind – тип колонки.
type ColumnKind string

const (
	ColumnGeneric  ColumnKind = ""
	ColumnPhone    ColumnKind = "phone"
	ColumnEmail    ColumnKind = "email"
	ColumnDiscount ColumnKind = "discount"
	ColumnDate     ColumnKind = "date"
)

// ClassifyHeader определяет назначение колонки по заголовку.
//
// Ожидаемые заголовки:
//
//	"Телефон"        → ColumnPhone
//	"Email"          → ColumnEmail
//	"E-mail"         → ColumnEmail
//	"Скидка"         → ColumnDiscount
//	"Дата Доставки"  → ColumnDate
//	всё остальное    → ColumnGeneric
func ClassifyHeader(header string) ColumnKind {
	switch header {
	case "Телефон":
		return ColumnPhone
	case "Email", "E-mail":
		return ColumnEmail
	case "Скидка":
		return ColumnDiscount
	case "Дата Доставки":
		return ColumnDate
	default:
		return ColumnGeneric
	}
}

// DetectPhoneColumn возвращает первый заголовок, распознанный как телефон.
func DetectPhoneColumn(headers []string) string {
	for _, header := range headers {
		if ClassifyHeader(header) == ColumnPhone {
			return header
		}
	}

	return ""
}
