package services

import (
	"fmt"
	"regexp"
	"strings"
)

var placeholderRegex = regexp.MustCompile(`\{\{(.+?)\}\}`)

// ParsePlaceholders возвращает уникальные непустые имена плейсхолдеров в порядке
// их первого появления в шаблоне.
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

// ValidateUnknownPlaceholders проверяет, что каждый плейсхолдер присутствует в
// списке допустимых полей.
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

// GenerateText подставляет значения строки в плейсхолдеры; неизвестные поля
// заменяются пустой строкой.
func GenerateText(template string, row map[string]string) string {
	return placeholderRegex.ReplaceAllStringFunc(template, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := row[name]; ok {
			return val
		}
		return ""
	})
}
