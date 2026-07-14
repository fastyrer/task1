package services

import (
	"reflect"
	"testing"

	"task1/models"
)

// TestRowToContact проверяет, что контакт содержит только фиксированные поля.
func TestRowToContact(t *testing.T) {
	row := map[string]string{
		"Телефон": "+7 (999) 123-45-67",
		"Имя":     "Анна",
		"Email":   "anna@example.com",
		"Скидка":  "15",
		"Город":   "Москва",
	}

	contact := RowToContact(row, row["Телефон"], "file-uid")
	if contact.Phone != row["Телефон"] || contact.Name != "Анна" {
		t.Fatalf("unexpected main contact fields: %#v", contact)
	}
	if contact.Email != "anna@example.com" || contact.Discount != "15" {
		t.Fatalf("unexpected normalized contact fields: %#v", contact)
	}
	if contact.FileID != "file-uid" {
		t.Fatalf("FileID = %q, want file-uid", contact.FileID)
	}

	typeOfContact := reflect.TypeOf(contact)
	if _, exists := typeOfContact.FieldByName("Data"); exists {
		t.Fatal("models.Contact must not contain Data")
	}
}

// TestContactIdentityFields фиксирует разделение внутреннего ID и публичного UID.
func TestContactIdentityFields(t *testing.T) {
	contact := models.Contact{ID: 42, UID: "2b352c2c-fbf3-40f2-8ac8-8ac7df40341c"}
	if contact.ID != 42 || contact.UID == "" {
		t.Fatalf("unexpected contact identity: %#v", contact)
	}
}
