package services

import (
	"context"
	"fmt"
	"strings"

	"task1/models"
	"task1/storage"
	"task1/utils"
)

type FixRowResult struct {
	Fixed  int                     `json:"fixed"`
	Failed []FixRowError           `json:"failed,omitempty"`
}

type FixRowError struct {
	RowNumber int                      `json:"rowNumber"`
	Errors    []models.ProcessingWarning `json:"errors"`
}

var ErrNoPhoneInRow = fmt.Errorf("в строке нет номера телефона")

type ProcessingResult struct {
	Saved      int                   `json:"saved"`
	Conflicts  []models.ConflictInfo `json:"conflicts"`
	Skipped    int                   `json:"skipped"`
}

func ProcessContacts(ctx context.Context, store storage.ContactStore, data models.FileData, phoneColumn string) (*ProcessingResult, error) {
	if phoneColumn == "" {
		return nil, fmt.Errorf("колонка с телефоном не найдена")
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

		contact := RowToContact(row, phone, data.ID)

		existing, exists, err := store.GetContactByPhone(ctx, phone)
		if err != nil {
			return nil, fmt.Errorf("check existing contact: %w", err)
		}

		if !exists {
			_, err := store.SaveContact(ctx, contact)
			if err != nil {
				return nil, fmt.Errorf("save contact: %w", err)
			}
			result.Saved++
			continue
		}

		if ContactsEqual(existing, contact) {
			result.Skipped++
			continue
		}

		conflict := detectConflict(i+1, existing, contact)
		result.Conflicts = append(result.Conflicts, conflict)
	}

	return result, nil
}

func RowToContact(row map[string]string, phone, fileID string) models.Contact {
	contact := models.Contact{
		Phone:  phone,
		FileID: fileID,
		Data:   make(map[string]string),
	}

	for header, value := range row {
		kind := utils.ClassifyHeader(header)
		value = strings.TrimSpace(value)

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

func isNameLikeField(header string) bool {
	key := utils.HeaderKey(header)
	switch key {
	case "имя", "фио", "name", "first name", "last name", "client", "клиент":
		return true
	default:
		return false
	}
}

func ContactsEqual(a, b models.Contact) bool {
	if a.Phone != b.Phone || a.Email != b.Email || a.Name != b.Name || a.Discount != b.Discount {
		return false
	}
	if len(a.Data) != len(b.Data) {
		return false
	}
	for k, v := range a.Data {
		if b.Data[k] != v {
			return false
		}
	}
	return true
}

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

func contactToMap(c models.Contact) map[string]string {
	m := make(map[string]string)
	m["phone"] = c.Phone
	m["email"] = c.Email
	m["name"] = c.Name
	m["discount"] = c.Discount
	for k, v := range c.Data {
		m[k] = v
	}
	return m
}

type FixRowInput struct {
	RowNumber int                `json:"rowNumber"`
	Values    map[string]string  `json:"values"`
}

func FixAndSaveRow(ctx context.Context, store storage.ContactStore, row FixRowInput, headers []string, phoneColumn string, fileID string) *FixRowError {
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
