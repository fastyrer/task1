package services

/*
	=======================
	Работа с плейсхолдерами
	=======================
*/

import (
	"fmt"
	"regexp"
	"strings"
)

// Переменные с основными ошибками
var (
	ErrEmptyTemplate = fmt.Errorf("Шаблон уведомления не может быть пустым")
	ErrEmptyPhone    = fmt.Errorf("Пустой номер телефона")
	ErrTooShortPhone = fmt.Errorf("Номер телефона слишком короткий (менее 7 цифр)")
)

// Регулярное выражение, MustCompile для безопасной инициализации
var placeholderRegex = regexp.MustCompile(`\{\{(.+?)\}\}`)

// ParsePlaceholders ищет все {{...}} в шаблоне, возвращает уникальные имена без
// повторений
func ParsePlaceholders(template string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(template, -1)
	seen := make(map[string]struct{}, len(matches))
	placeholders := make([]string, 0, len(matches))

	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		placeholders = append(placeholders, name)
	}

	return placeholders
}

// ValidateUnknownPlaceholders строит map из заголовков файла, сверяет все
// плейсхолдеры с ними. Если нет соответствия, то собирает список неизвестных
// и возвращает ошибку
func ValidateUnknownPlaceholders(placeholders, headers []string) error {
	headerSet := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		headerSet[h] = struct{}{}
	}

	var unknown []string
	for _, p := range placeholders {
		if _, ok := headerSet[p]; !ok {
			unknown = append(unknown, p)
		}
	}

	if len(unknown) > 0 {
		return fmt.Errorf("Неизвестные плейсхолдеры: %s.", strings.Join(unknown, ", "))
	}

	return nil
}

// GenerateText Для каждого {{Name}} в шаблоне достаёт значение из row[Name] 
// и подставляет в текст. 
// Если заголовок в строке отсутствует — подставляет пустую строку.
func GenerateText(template string, row map[string]string) string {
	return placeholderRegex.ReplaceAllStringFunc(template, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := row[name]; ok {
			return val
		}
		return ""
	})
}
