// file_schema.go - единый контракт обязательных колонок импортируемой таблицы.
package services

import (
	"errors"
	"fmt"
	"strings"

	"task1/models"
	"task1/utils"
)

var ErrMissingRequiredColumns = errors.New("Таблица не соответствует формату")

type contactColumnRequirement struct {
	Kind          utils.ColumnKind
	Label         string
	ValueRequired bool
}

// contactColumnRequirements задаёт структуру файла в одном месте.
// Email и скидка могут быть пустыми в строке, но сами колонки обязательны.
var contactColumnRequirements = []contactColumnRequirement{
	{Kind: utils.ColumnPhone, Label: "Телефон", ValueRequired: true},
	{Kind: utils.ColumnName, Label: "ФИО", ValueRequired: true},
	{Kind: utils.ColumnEmail, Label: "Email"},
	{Kind: utils.ColumnDiscount, Label: "Скидка"},
}

// validateRequiredContactColumns отклоняет таблицу целиком, если в ней нет
// хотя бы одной обязательной смысловой колонки.
func validateRequiredContactColumns(headers []string) error {
	present := make(map[utils.ColumnKind]bool, len(contactColumnRequirements))
	for _, header := range headers {
		present[utils.ClassifyHeader(header)] = true
	}

	missing := make([]string, 0, len(contactColumnRequirements))
	for _, requirement := range contactColumnRequirements {
		if !present[requirement.Kind] {
			missing = append(missing, fmt.Sprintf("«%s»", requirement.Label))
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("%w. Отсутствуют обязательные колонки: %s.", ErrMissingRequiredColumns, strings.Join(missing, ", "))
}

// requiredValueHeaders возвращает реальные заголовки колонок, которые нельзя
// оставлять пустыми в отдельной строке.
func requiredValueHeaders(headers []string) []string {
	headerByKind := make(map[utils.ColumnKind]string, len(contactColumnRequirements))
	for _, header := range headers {
		kind := utils.ClassifyHeader(header)
		if _, exists := headerByKind[kind]; !exists {
			headerByKind[kind] = header
		}
	}

	required := make([]string, 0, len(contactColumnRequirements))
	for _, requirement := range contactColumnRequirements {
		if requirement.ValueRequired && headerByKind[requirement.Kind] != "" {
			required = append(required, headerByKind[requirement.Kind])
		}
	}
	return required
}

// tableValidationError выделяет случай, когда структура допустима, но ни одна
// строка пока не может быть импортирована.
func tableValidationError(stats models.ProcessingStats) string {
	if stats.RowCount > 0 && stats.ValidRowCount == 0 {
		return "В таблице нет ни одной записи, готовой к импорту. Исправьте строки с ошибками."
	}
	return ""
}
