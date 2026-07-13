// Package utils содержит чистые функции без состояния и побочных эффектов.
//
// columns.go – классификация колонок по их заголовкам.
// Определяет тип колонки (телефон, email, скидка, дата) по тексту заголовка,
// поддерживает русские и английские названия, регистронезависимо,
// Используется при парсинге файлов (для валидации)
// и в обработчиках (для поиска колонки телефона).
package utils

import "strings"

// ColumnKind – тип колонки, определённый по её заголовку.
// Пустая строка (ColumnGeneric) означает «не распознана».
type ColumnKind string

const (
	ColumnGeneric  ColumnKind = ""
	ColumnPhone    ColumnKind = "phone"
	ColumnEmail    ColumnKind = "email"
	ColumnDiscount ColumnKind = "discount"
	ColumnDate     ColumnKind = "date"
)

// ClassifyHeader определяет назначение колонки по её заголовку.
func ClassifyHeader(header string) ColumnKind {
	// 1. Приведение заголовка к единому виду
	key := HeaderKey(header)

	// 2. Определение типа колонки
	switch {
	case strings.Contains(key, "телефон"), key == "phone", strings.Contains(key, "mobile"):
		return ColumnPhone
	case key == "email", key == "e-mail", strings.Contains(key, "почта"), strings.Contains(key, "mail"):
		return ColumnEmail
	case strings.Contains(key, "скидка"), strings.Contains(key, "discount"), strings.Contains(key, "процент"):
		return ColumnDiscount
	case strings.Contains(key, "дата"), key == "date", strings.HasSuffix(key, " date"):
		return ColumnDate
	default:
		return ColumnGeneric
	}
}

// IsCommonHeader сообщает, похож ли заголовок на распространённый (не технический).
//
// Считаются распространёнными:
//   - заголовки, распознанные ClassifyHeader (телефон, email, скидка, дата)
//   - заголовки имён, ФИО, клиента, города (рус/англ)
//
// Используется в header_detector скоринга: чем больше common-заголовков в строке,
// тем вероятнее, что это строка-заголовок, а не данные.
func IsCommonHeader(header string) bool {
	key := HeaderKey(header)
	if ClassifyHeader(header) != ColumnGeneric {
		return true
	}

	switch key {
	case "имя", "фио", "name", "first name", "last name", "клиент", "client", "город", "city":
		return true
	default:
		return false
	}
}

// HeaderKey приводит заголовок к форме, пригодной для сравнения.
//
// Преобразования:
//   - Удаление BOM и лишних пробелов (через CleanHeader)
//   - Нижний регистр
//   - Замена "ё" → "е"
//
// Нужна, чтобы "Телефон" и "телефонъ" (опечатка с твёрдым знаком, если бы не clean)
// или "Ёмейл" и "емейл" считались одинаковыми.
func HeaderKey(header string) string {
	key := strings.ToLower(CleanHeader(header))
	return strings.ReplaceAll(key, "ё", "е")
}

// DetectPhoneColumn возвращает первый заголовок, распознанный как телефонная колонка.
//
// Проходит по slice заголовков, вызывает ClassifyHeader на каждом.
// Возвращает пустую строку, если ни одна колонка не похожа на телефон.
//
// Вызывается:
//   - В upload.go – чтобы вернуть detectedPhoneColumn фронту
//   - В contact.go – чтобы найти колонку для сохранения контактов
//   - В notification.go – чтобы найти колонку для генерации уведомлений
func DetectPhoneColumn(headers []string) string {
	for _, header := range headers {
		if ClassifyHeader(header) == ColumnPhone {
			return header
		}
	}

	return ""
}
