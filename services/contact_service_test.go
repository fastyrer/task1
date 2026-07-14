package services

import (
	"context"
	"reflect"
	"testing"

	"task1/models"
	"task1/storage"
)

type contactStoreStub struct {
	saveErr       error
	existing      models.Contact
	existingFound bool
	matched       int
	fixedFileID   string
	fixedRow      int
	fixedValues   map[string]string
	fixedContact  models.Contact
}

func (s *contactStoreStub) SaveContact(context.Context, models.Contact) (string, error) {
	return "", s.saveErr
}

func (s *contactStoreStub) ListContacts(context.Context) ([]models.Contact, error) {
	return nil, nil
}

func (s *contactStoreStub) GetContactByPhone(context.Context, string) (models.Contact, bool, error) {
	return s.existing, s.existingFound, nil
}

func (s *contactStoreStub) RecordContactMatch(context.Context, models.Contact, models.Contact) error {
	s.matched++
	return nil
}

func (s *contactStoreStub) ResolveConflict(context.Context, string, models.ConflictAction, models.Contact) error {
	return nil
}

func (s *contactStoreStub) SaveFixedRow(
	_ context.Context,
	fileID string,
	rowNumber int,
	values map[string]string,
	contact models.Contact,
) error {
	s.fixedFileID = fileID
	s.fixedRow = rowNumber
	s.fixedValues = values
	s.fixedContact = contact
	return nil
}

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

func TestProcessContactsRecordsMatchedSource(t *testing.T) {
	phone := "+7 (999) 123-45-67"
	store := &contactStoreStub{
		saveErr:       storage.ErrContactAlreadyExists,
		existingFound: true,
		existing: models.Contact{
			ID:    10,
			UID:   "2b352c2c-fbf3-40f2-8ac8-8ac7df40341c",
			Phone: phone,
			Name:  "Анна",
		},
	}
	data := models.FileData{
		ID:         "file-2",
		Headers:    []string{"Телефон", "Имя"},
		Rows:       []map[string]string{{"Телефон": phone, "Имя": "Анна"}},
		RowNumbers: []int{7},
	}

	result, err := ProcessContacts(context.Background(), store, data, "Телефон")
	if err != nil {
		t.Fatalf("ProcessContacts: %v", err)
	}
	if result.Skipped != 1 || store.matched != 1 {
		t.Fatalf("Skipped = %d, matched calls = %d", result.Skipped, store.matched)
	}
}

func TestFixAndSaveRowUsesAtomicStoreOperation(t *testing.T) {
	store := &contactStoreStub{}
	row := FixRowInput{
		RowNumber: 4,
		Values: map[string]string{
			"Телефон": "89991234567",
			"Имя":     " Анна ",
			"Email":   "ANNA@EXAMPLE.COM",
			"Скидка":  "15%",
		},
	}

	if fixErr := FixAndSaveRow(context.Background(), store, row, "Телефон", "file-1"); fixErr != nil {
		t.Fatalf("FixAndSaveRow: %#v", fixErr)
	}
	if store.fixedFileID != "file-1" || store.fixedRow != 4 {
		t.Fatalf("unexpected fixed row target: %q row %d", store.fixedFileID, store.fixedRow)
	}
	if store.fixedValues["Телефон"] != "+7 (999) 123-45-67" {
		t.Fatalf("phone was not normalized: %q", store.fixedValues["Телефон"])
	}
	if store.fixedContact.Email != "anna@example.com" || store.fixedContact.Discount != "15" {
		t.Fatalf("unexpected fixed contact: %#v", store.fixedContact)
	}
}
