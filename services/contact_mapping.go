// contact_mapping.go - чистое преобразование строки файла в фиксированную модель контакта.
package services

import (
	"strings"
	"time"

	"task1/models"
	"task1/utils"
)

// RowToContact выбирает из строки только поля рабочего справочника.
// Произвольные колонки остаются в подтверждённом file_rows и не попадают в contacts.
func RowToContact(row map[string]string, phone, fileID string) models.Contact {
	contact := models.Contact{Phone: phone, FileID: fileID}
	for header, value := range row {
		value = strings.TrimSpace(value)
		switch utils.ClassifyHeader(header) {
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

// ContactsEqual сравнивает только актуальные поля таблицы contacts.
func ContactsEqual(a, b models.Contact) bool {
	return a.Phone == b.Phone &&
		a.Email == b.Email &&
		a.Name == b.Name &&
		a.Discount == b.Discount
}

// detectConflict формирует read-only описание расхождений для frontend.
func detectConflict(rowNumber int, existing, incoming models.Contact) models.ConflictInfo {
	differences := make([]string, 0, 3)
	if existing.Name != incoming.Name {
		differences = append(differences, "name")
	}
	if existing.Email != incoming.Email {
		differences = append(differences, "email")
	}
	if existing.Discount != incoming.Discount {
		differences = append(differences, "discount")
	}

	return models.ConflictInfo{
		Row:         rowNumber,
		Phone:       incoming.Phone,
		Version:     existing.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Existing:    contactToMap(existing),
		Incoming:    contactToMap(incoming),
		Differences: differences,
		Actions: []models.ConflictAction{
			models.ConflictActionSkip,
			models.ConflictActionReplace,
			models.ConflictActionMerge,
		},
	}
}

func contactToMap(contact models.Contact) map[string]string {
	return map[string]string{
		"phone":    contact.Phone,
		"email":    contact.Email,
		"name":     contact.Name,
		"discount": contact.Discount,
	}
}
