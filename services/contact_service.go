package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task1/models"
	"task1/storage"
	"task1/utils"
)

type FixRowResult struct {
	Fixed    int                        `json:"fixed" example:"2"`
	Failed   []FixRowError              `json:"failed,omitempty"`
	Stats    *models.ProcessingStats    `json:"stats,omitempty"`
	Warnings []models.ProcessingWarning `json:"warnings,omitempty"`
}

type FixRowError struct {
	RowNumber int                        `json:"rowNumber" example:"4"`
	Errors    []models.ProcessingWarning `json:"errors"`
}

// ProcessingResult – итог обработки набора контактов.
type ProcessingResult struct {
	Saved     int                   `json:"saved"`
	Conflicts []models.ConflictInfo `json:"conflicts"`
	Skipped   int                   `json:"skipped"`
}

// ProcessContacts – обрабатывает строки файла, пропускает пустые и помеченные как invalid записи,
// сохраняет новые контакты и формирует список конфликтов при несовпадении данных.

// ProcessContacts:
// 1. Проверка наличия колонки с телефоном
// 2. Создание набора invalid строк для быстрого поиска
// 3. Инициализация результата обработки
// 4. Обход всех строк файла
// 5. Проверка телефона и пропуск пустых/invalid строк
// 6. Сохранение новых контактов или формирование конфликтов
func ProcessContacts(ctx context.Context, store storage.ContactStore, data models.FileData, phoneColumn string) (*ProcessingResult, error) {

	// 1. Проверка наличия колонки с телефоном
	if phoneColumn == "" {
		return nil, fmt.Errorf(ErrorPhoneColNotFound)
	}

	invalidSet := make(map[int]struct{}, len(data.InvalidRows))
	for _, inv := range data.InvalidRows {
		invalidSet[inv.Row] = struct{}{}
	}

	result := &ProcessingResult{}

	for i, row := range data.Rows {
		phone := strings.TrimSpace(row[phoneColumn])
		if phone == "" {
			result.Skipped++
			continue
		}

		if i < len(data.RowNumbers) {
			if _, ok := invalidSet[data.RowNumbers[i]]; ok {
				result.Skipped++
				continue
			}
		}

		// 6. Создание объекта контакта из строки.
		// Для связи с источником берём исходный номер, а не индекс в Go-slice.
		rowNumber := i + 1
		if i < len(data.RowNumbers) && data.RowNumbers[i] > 0 {
			rowNumber = data.RowNumbers[i]
		}
		contact := RowToContact(row, phone, data.ID)
		contact.SourceRow = rowNumber

		// INSERT ... ON CONFLICT в PostgreSQL атомарно определяет, новый ли это контакт.
		_, err := store.SaveContact(ctx, contact)
		if err == nil {
			result.Saved++
			continue
		}
		if !errors.Is(err, storage.ErrContactAlreadyExists) {
			return nil, fmt.Errorf("save contact: %w", err)
		}

		// SaveContact сообщил о UNIQUE-конфликте: загружаем текущие данные для сравнения.
		existing, exists, err := store.GetContactByPhone(ctx, phone)
		if err != nil {
			return nil, fmt.Errorf("load conflicting contact: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("load conflicting contact: %w", storage.ErrContactNotFound)
		}

		if ContactsEqual(existing, contact) {
			if err := store.RecordContactMatch(ctx, existing, contact); err != nil {
				return nil, fmt.Errorf("record matching contact: %w", err)
			}
			result.Saved++
			continue
		}

		// 6. Формирование конфликта и добавление его в результат
		conflict := detectConflict(rowNumber, existing, contact)
		result.Conflicts = append(result.Conflicts, conflict)
	}

	return result, nil
}

func RowToContact(row map[string]string, phone, fileID string) models.Contact {
	contact := models.Contact{
		Phone:  phone,
		FileID: fileID,
	}

	for header, value := range row {
		kind := utils.ClassifyHeader(header)
		value = strings.TrimSpace(value)

		switch kind {
		case utils.ColumnPhone:
			continue
		case utils.ColumnName:
			contact.Name = value
		case utils.ColumnEmail:
			contact.Email = value
		case utils.ColumnDiscount:
			contact.Discount = value
		}
	}

	return contact
}

// ContactsEqual – сравнивает два контакта по основным полям.

// ContactsEqual сравнивает фиксированные поля контакта.
func ContactsEqual(a, b models.Contact) bool {
	return a.Phone == b.Phone &&
		a.Email == b.Email &&
		a.Name == b.Name &&
		a.Discount == b.Discount
}

// detectConflict – формирует описание конфликта между существующим и входящим контактом.

// detectConflict:
// 1. Преобразование контактов в удобный вид для сравнения
// 2. Поиск различий по фиксированным полям (name, email, discount)
// 3. Формирование информации о конфликте и доступных действиях
func detectConflict(rowNum int, existing, incoming models.Contact) models.ConflictInfo {
	existingMap := contactToMap(existing)
	incomingMap := contactToMap(incoming)

	differences := make([]string, 0)
	if existing.Name != incoming.Name {
		differences = append(differences, "name")
	}
	if existing.Email != incoming.Email {
		differences = append(differences, "email")
	}
	if existing.Discount != incoming.Discount {
		differences = append(differences, "discount")
	}

	// 3. Формирование информации о конфликте и доступных действиях
	return models.ConflictInfo{
		Row:         rowNum,
		Phone:       incoming.Phone,
		Existing:    existingMap,
		Incoming:    incomingMap,
		Differences: differences,
		Actions: []models.ConflictAction{
			models.ConflictActionSkip,
			models.ConflictActionReplace,
			models.ConflictActionMerge,
		},
	}
}

// contactToMap – преобразует контакт в map для сравнения и формирования ответа.

// contactToMap:
// 1. Создание пустой карты значений
// 2. Заполнение основных полей контакта
func contactToMap(c models.Contact) map[string]string {
	m := make(map[string]string, 4)
	m["phone"] = c.Phone
	m["email"] = c.Email
	m["name"] = c.Name
	m["discount"] = c.Discount
	return m
}

type FixRowInput struct {
	RowNumber int               `json:"rowNumber" validate:"required" example:"4"`
	Values    map[string]string `json:"values" validate:"required"`
}

// FixAndSaveRow – валидирует и сохраняет исправленную строку, учитывая возможные конфликты.

// FixAndSaveRow:
// 1. Очистка и нормализация значений строки
// 2. Проверка корректности телефона, email, скидки и даты
// 3. Сохранение контакта или формирование ошибки/конфликта
func FixAndSaveRow(ctx context.Context, store storage.ContactStore, row FixRowInput, phoneColumn string, fileID string) *FixRowError {
	rowErrors := make([]models.ProcessingWarning, 0)
	values := make(map[string]string)

	for header, value := range row.Values {
		value = strings.TrimSpace(value)
		kind := utils.ClassifyHeader(header)
		cleaned := utils.CleanCell(value)

		if cleaned == "" {
			values[header] = ""
			continue
		}

		switch kind {
		case utils.ColumnPhone:
			normalized, ok := utils.NormalizePhone(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: ErrorPhoneIncorrect,
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized
		case utils.ColumnEmail:
			normalized, ok := utils.NormalizeEmail(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: ErrorEmailIncorrect,
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized
		case utils.ColumnDiscount:
			normalized, ok := utils.NormalizePercent(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: ErrorDiscountIncorrect,
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized
		default:
			values[header] = cleaned
		}
	}

	phone := strings.TrimSpace(values[phoneColumn])
	if phone == "" {
		rowErrors = append(rowErrors, models.ProcessingWarning{
			Row: row.RowNumber, Column: phoneColumn, Message: ErrorPhoneEmpty,
		})
	}

	if len(rowErrors) > 0 {
		return &FixRowError{
			RowNumber: row.RowNumber,
			Errors:    rowErrors,
		}
	}

	// 3. Строка файла, контакт и связь с источником сохраняются одной транзакцией PostgreSQL.
	contact := RowToContact(values, phone, fileID)
	contact.SourceRow = row.RowNumber
	if err := store.SaveFixedRow(ctx, fileID, row.RowNumber, values, contact); err != nil {
		return &FixRowError{
			RowNumber: row.RowNumber,
			Errors:    []models.ProcessingWarning{{Message: fmt.Sprintf("Ошибка сохранения: %v", err)}},
		}
	}

	return nil
}
