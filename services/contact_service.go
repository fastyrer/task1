// Package services содержит бизнес-логику обработки контактов, конфликтов и исправления строк.

// contact_service.go – преобразование данных из файла в контакты, проверка на дубликаты,
// сохранение новых записей и формирование конфликтов при несовпадении данных.

package services

import (
	"context"
	"fmt"
	"strings"

	"task1/models"
	"task1/storage"
	"task1/utils"
)

// FixRowResult – результат исправления одной строки.
type FixRowResult struct {
	Fixed  int                     `json:"fixed"`
	Failed []FixRowError           `json:"failed,omitempty"`
}

// FixRowError – ошибки, возникшие при исправлении строки.
type FixRowError struct {
	RowNumber int                      `json:"rowNumber"`
	Errors    []models.ProcessingWarning `json:"errors"`
}

var ErrNoPhoneInRow = fmt.Errorf("в строке нет номера телефона")

// ProcessingResult – итог обработки набора контактов.
type ProcessingResult struct {
	Saved      int                   `json:"saved"`
	Conflicts  []models.ConflictInfo `json:"conflicts"`
	Skipped    int                   `json:"skipped"`
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
		return nil, fmt.Errorf("колонка с телефоном не найдена")
	}

	// 2. Создание набора invalid строк для быстрого поиска
	invalidSet := make(map[int]struct{}, len(data.InvalidRows))
	for _, inv := range data.InvalidRows {
		invalidSet[inv.Row] = struct{}{}
	}

	// 3. Инициализация результата обработки
	result := &ProcessingResult{}

	// 4. Обход всех строк файла
	for i, row := range data.Rows {

		// 5. Проверка наличия колонки с телефоном в текущей строке
		phone := strings.TrimSpace(row[phoneColumn])
		if phone == "" {
			result.Skipped++
			continue
		}

		// 5. Проверка, что строка не помечена как invalid
		if i < len(data.RowNumbers) {
			if _, ok := invalidSet[data.RowNumbers[i]]; ok {
				result.Skipped++
				continue
			}
		}

		// 6. Создание объекта контакта из строки
		contact := RowToContact(row, phone, data.ID)

		// Проверка наличия существующего контакта
		existing, exists, err := store.GetContactByPhone(ctx, phone)
		if err != nil {
			return nil, fmt.Errorf("check existing contact: %w", err)
		}

		// Сохранение нового контакта
		if !exists {
			_, err := store.SaveContact(ctx, contact)
			if err != nil {
				return nil, fmt.Errorf("save contact: %w", err)
			}
			result.Saved++
			continue
		}

		// Проверка на совпадение данных
		if ContactsEqual(existing, contact) {
			result.Skipped++
			continue
		}

		// 6. Формирование конфликта и добавление его в результат
		conflict := detectConflict(i+1, existing, contact)
		result.Conflicts = append(result.Conflicts, conflict)
	}

	return result, nil
}

// RowToContact – преобразует строку из файла в объект контакта.

// RowToContact:
// 1. Создание базового объекта контакта
// 2. Обход всех значений строки
// 3. Определение типа колонки и распределение данных по полям
func RowToContact(row map[string]string, phone, fileID string) models.Contact {
	// 1. Создание базового объекта контакта
	contact := models.Contact{
		Phone:  phone,
		FileID: fileID,
		Data:   make(map[string]string),
	}

	// 2. Обход всех значений строки
	for header, value := range row {
		kind := utils.ClassifyHeader(header)
		value = strings.TrimSpace(value)

		// 3. Определение типа колонки и распределение данных по полям
		switch kind {
		case utils.ColumnPhone:
			continue
		case utils.ColumnEmail:
			contact.Email = value
		case utils.ColumnDiscount:
			contact.Discount = value
		case utils.ColumnGeneric:
			if isNameLikeField(header) {
				contact.Name = value
			} else {
				contact.Data[header] = value
			}
		default:
			contact.Data[header] = value
		}
	}

	return contact
}

// isNameLikeField – определяет, относится ли колонка к имени клиента.

// isNameLikeField:
// 1. Нормализация имени колонки
// 2. Сопоставление с известными вариантами имени
func isNameLikeField(header string) bool {
	switch header {
	case "Имя", "Фамилия", "ФИО", "Name", "First name", "Last name", "Client", "Клиент":
		return true
	default:
		return false
	}
}

// ContactsEqual – сравнивает два контакта по основным полям.

// ContactsEqual:
// 1. Сравнение основных полей контакта
// 2. Проверка количества дополнительных данных
// 3. Сравнение значений дополнительных полей
func ContactsEqual(a, b models.Contact) bool {
	// 1. Сравнение основных полей контакта
	if a.Phone != b.Phone || a.Email != b.Email || a.Name != b.Name || a.Discount != b.Discount {
		return false
	}

	// 2. Проверка количества дополнительных данных
	if len(a.Data) != len(b.Data) {
		return false
	}

	// 3. Сравнение значений дополнительных полей
	for k, v := range a.Data {
		if b.Data[k] != v {
			return false
		}
	}
	return true
}

// detectConflict – формирует описание конфликта между существующим и входящим контактом.

// detectConflict:
// 1. Преобразование контактов в удобный вид для сравнения
// 2. Поиск различий по основным полям (name, email, discount) и дополнительным данным
// 3. Формирование информации о конфликте и доступных действиях
func detectConflict(rowNum int, existing, incoming models.Contact) models.ConflictInfo {
	// 1. Преобразование контактов в удобный вид для сравнения
	existingMap := contactToMap(existing)
	incomingMap := contactToMap(incoming)

	// 2. Поиск различий по основным полям
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

	// 2. Поиск различий по дополнительным полям
	for k, v := range existing.Data {
		if incoming.Data[k] != v {
			differences = append(differences, k)
		}
	}
	for k, v := range incoming.Data {
		if existing.Data[k] != v {
			differences = append(differences, k)
		}
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
// 3. Добавление дополнительных данных из contact.Data

func contactToMap(c models.Contact) map[string]string {
	// 1. Создание пустой карты значений
	m := make(map[string]string)
	// 2. Заполнение основных полей контакта
	m["phone"] = c.Phone
	m["email"] = c.Email
	m["name"] = c.Name
	m["discount"] = c.Discount
	// 3. Добавление дополнительных данных из contact.Data
	for k, v := range c.Data {
		m[k] = v
	}
	return m
}

// FixRowInput – данные строки, которую нужно исправить и сохранить.
type FixRowInput struct {
	RowNumber int                `json:"rowNumber"`
	Values    map[string]string  `json:"values"`
}

// FixAndSaveRow – валидирует и сохраняет исправленную строку, учитывая возможные конфликты.

// FixAndSaveRow:
// 1. Очистка и нормализация значений строки
// 2. Проверка корректности телефона, email, скидки и даты
// 3. Сохранение контакта или формирование ошибки/конфликта
func FixAndSaveRow(ctx context.Context, store storage.ContactStore, row FixRowInput, headers []string, phoneColumn string, fileID string) *FixRowError {
	rowErrors := make([]models.ProcessingWarning, 0)
	values := make(map[string]string)

	// 1. Очистка и нормализация значений строки
	for header, value := range row.Values {
		value = strings.TrimSpace(value)
		kind := utils.ClassifyHeader(header)
		cleaned := utils.CleanCell(value)

		if cleaned == "" {
			values[header] = ""
			continue
		}

		// 2. Проверка корректности телефона, email, скидки и даты
		switch kind {

		case utils.ColumnPhone:
			normalized, ok := utils.NormalizePhone(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: "Некорректный телефон.",
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized

		case utils.ColumnEmail:
			normalized, ok := utils.NormalizeEmail(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: "Некорректный email.",
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized

		case utils.ColumnDiscount:
			normalized, ok := utils.NormalizePercent(cleaned)
			if !ok {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: "Скидка должна быть числом от 0 до 100.",
				})
				values[header] = cleaned
				continue
			}
			values[header] = normalized

		case utils.ColumnDate:
			if !utils.IsSupportedDate(cleaned) {
				rowErrors = append(rowErrors, models.ProcessingWarning{
					Row: row.RowNumber, Column: header, Message: "Дата должна быть в распознаваемом формате.",
				})
				values[header] = cleaned
				continue
			}
			values[header] = cleaned

		default:
			values[header] = cleaned
		}
	}

	phone := strings.TrimSpace(values[phoneColumn])
	if phone == "" {
		rowErrors = append(rowErrors, models.ProcessingWarning{
			Row: row.RowNumber, Column: phoneColumn, Message: "Номер телефона обязателен.",
		})
	}

	if len(rowErrors) > 0 {
		return &FixRowError{
			RowNumber: row.RowNumber,
			Errors:    rowErrors,
		}
	}

	// 3. Сохранение контакта или формирование ошибки/конфликта
	contact := RowToContact(values, phone, fileID)
	existing, exists, err := store.GetContactByPhone(ctx, phone)
	if err != nil {
		return &FixRowError{
			RowNumber: row.RowNumber,
			Errors: []models.ProcessingWarning{{Message: fmt.Sprintf("Ошибка БД: %v", err)}},
		}
	}

	if exists && !ContactsEqual(existing, contact) {
		if err := store.ResolveConflict(ctx, phone, models.ConflictActionReplace, contact); err != nil {
			return &FixRowError{
				RowNumber: row.RowNumber,
				Errors:    []models.ProcessingWarning{{Message: fmt.Sprintf("Ошибка сохранения: %v", err)}},
			}
		}
		return nil
	}

	if _, err := store.SaveContact(ctx, contact); err != nil {
		return &FixRowError{
			RowNumber: row.RowNumber,
			Errors:    []models.ProcessingWarning{{Message: fmt.Sprintf("Ошибка сохранения: %v", err)}},
		}
	}

	return nil
}
