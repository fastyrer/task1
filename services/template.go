// Package services содержит утилиты для работы с файлами и шаблонами.
//
// template.go – парсит плейсхолдеры в шаблоне, проверяет их наличие в заголовках
// и генерирует финальный текст на основе данных строки.

package services

import (
	"fmt"
	"regexp"
	"strings"
)

// placeholderRegex – регулярное выражение для поиска плейсхолдеров вида {{name}}.
var placeholderRegex = regexp.MustCompile(`\{\{(.+?)\}\}`)

// ParsePlaceholders находит все уникальные плейсхолдеры в шаблоне.

// ParsePlaceholders:
// 1. Ищет все совпадения {{...}} в тексте шаблона
// 2. Обрезает пробелы вокруг имени плейсхолдера
// 3. Пропускает пустые имена и дубликаты
func ParsePlaceholders(template string) []string {

	// 1. Ищем все совпадения {{...}} в тексте шаблона
	matches := placeholderRegex.FindAllStringSubmatch(template, -1)
	seen := make(map[string]struct{}, len(matches))
	placeholders := make([]string, 0, len(matches))

	for _, m := range matches {
		// 2. Обрезаем пробелы вокруг имени плейсхолдера
		name := strings.TrimSpace(m[1])
		if name == "" {
			// 3. Пропускаем пустые имена
			continue
		}
		if _, ok := seen[name]; ok {
			// 3. Пропускаем дубликаты
			continue
		}
		seen[name] = struct{}{}
		placeholders = append(placeholders, name)
	}

	return placeholders
}

// ValidateUnknownPlaceholders проверяет, что все плейсхолдеры присутствуют в заголовках.

// ValidateUnknownPlaceholders:
// 1. Преобразует список заголовков в set
// 2. Проходит по всем плейсхолдерам и находит неизвестные
// 3. Возвращает ошибку с перечислением неизвестных плейсхолдеров
func ValidateUnknownPlaceholders(placeholders, headers []string) error {
	// 1. Преобразует список заголовков в set
	headerSet := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		headerSet[h] = struct{}{}
	}

	var unknown []string
	// 2. Проходит по всем плейсхолдерам и находит неизвестные
	for _, p := range placeholders {
		if _, ok := headerSet[p]; !ok {
			unknown = append(unknown, p)
		}
	}

	// 3. Возвращает ошибку с перечислением неизвестных плейсхолдеров
	if len(unknown) > 0 {
		return fmt.Errorf("Неизвестные плейсхолдеры: %s.", strings.Join(unknown, ", "))
	}

	return nil
}

// GenerateText формирует итоговый текст, подставляя значения из строки в плейсхолдеры.

// GenerateText:
// 1. Ищет все плейсхолдеры {{name}} в шаблоне
// 2. Берёт значение из row по имени плейсхолдера
// 3. Если значение отсутствует — подставляет пустую строку
func GenerateText(template string, row map[string]string) string {
	return placeholderRegex.ReplaceAllStringFunc(template, func(match string) string {
		// 1. Извлекаем имя плейсхолдера без {{ }}
		name := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := row[name]; ok {
			// 2. Возвращаем значение из строки для найденного имени
			return val
		}
		// 3. Если значение отсутствует — возвращаем пустую строку
		return ""
	})
}
