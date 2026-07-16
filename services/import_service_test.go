package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"task1/models"
)

const testImportID = "550e8400-e29b-41d4-a716-446655440000"

type contactReaderStub struct {
	contacts map[string]models.Contact
	phones   []string
}

func (stub *contactReaderStub) FindContactsByPhones(_ context.Context, phones []string) (map[string]models.Contact, error) {
	stub.phones = append([]string(nil), phones...)
	return stub.contacts, nil
}

func TestValidateImportDraftNormalizesEditedRows(t *testing.T) {
	draft := validDraft()
	draft.Rows[0].Values["Телефон"] = "8 (999) 123-45-67"
	draft.Rows[0].Values["Email"] = " USER@Example.COM "

	data, err := ValidateImportDraft(draft)
	if err != nil {
		t.Fatalf("ValidateImportDraft() error = %v", err)
	}
	if got := data.Rows[0]["Телефон"]; got != "+79991234567" {
		t.Fatalf("normalized phone = %q", got)
	}
	if got := data.Rows[0]["Email"]; got != "user@example.com" {
		t.Fatalf("normalized email = %q", got)
	}
	if data.Stats.InvalidRowCount != 0 || data.Stats.ValidRowCount != 1 {
		t.Fatalf("unexpected stats: %+v", data.Stats)
	}
}

func TestPreviewImportRejectsInvalidDraft(t *testing.T) {
	data, err := ValidateImportDraft(models.ImportDraft{
		ImportID:         testImportID,
		OriginalFilename: "invalid.csv",
		Format:           "csv",
		HeaderRow:        1,
		Headers:          []string{"Телефон", "Email"},
		Rows: []models.ImportRow{{
			RowNumber: 2,
			Values: map[string]string{
				"Телефон": "+79991234567",
				"Email":   "not-an-email",
			},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateImportDraft() error = %v", err)
	}

	_, err = PreviewImport(context.Background(), &contactReaderStub{}, data)
	if !errors.Is(err, ErrDraftHasInvalidRows) {
		t.Fatalf("PreviewImport() error = %v, want %v", err, ErrDraftHasInvalidRows)
	}
}

func TestPreviewImportClassifiesContacts(t *testing.T) {
	updatedAt := time.Date(2026, 7, 16, 8, 30, 0, 123456000, time.UTC)
	data := models.FileData{
		ID:      testImportID,
		Headers: []string{"Телефон", "ФИО", "Email"},
		Rows: []map[string]string{
			{"Телефон": "+79991111111", "ФИО": "Новый"},
			{"Телефон": "+79992222222", "ФИО": "Совпадает", "Email": "same@example.com"},
			{"Телефон": "+79993333333", "ФИО": "Новое имя"},
		},
		RowNumbers: []int{2, 3, 4},
	}
	reader := &contactReaderStub{contacts: map[string]models.Contact{
		"+79992222222": {
			Phone: "+79992222222", Name: "Совпадает", Email: "same@example.com", UpdatedAt: updatedAt,
		},
		"+79993333333": {
			Phone: "+79993333333", Name: "Старое имя", UpdatedAt: updatedAt,
		},
	}}

	preview, err := PreviewImport(context.Background(), reader, data)
	if err != nil {
		t.Fatalf("PreviewImport() error = %v", err)
	}
	if preview.NewCount != 1 || preview.MatchedCount != 1 || preview.ConflictCount != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if len(reader.phones) != 3 {
		t.Fatalf("database lookup received %d phones", len(reader.phones))
	}
	if preview.Conflicts[0].Row != 4 || preview.Conflicts[0].Version == "" {
		t.Fatalf("unexpected conflict: %+v", preview.Conflicts[0])
	}
}

func TestPrepareImportRequiresCurrentConflictVersion(t *testing.T) {
	data := models.FileData{
		ID:         testImportID,
		Headers:    []string{"Телефон", "ФИО"},
		Rows:       []map[string]string{{"Телефон": "+79993333333", "ФИО": "Новое имя"}},
		RowNumbers: []int{2},
	}
	preview := models.ImportPreviewResult{
		ConflictCount: 1,
		Conflicts: []models.ConflictInfo{{
			Phone: "+79993333333", Version: "2026-07-16T08:30:00Z",
		}},
	}

	_, _, err := PrepareImport(data, preview, nil)
	if !errors.Is(err, ErrDecisionMissing) {
		t.Fatalf("missing decision error = %v", err)
	}

	_, _, err = PrepareImport(data, preview, []models.ImportDecision{{
		Phone: "+79993333333", Action: models.ConflictActionReplace, Version: "old",
	}})
	if !errors.Is(err, ErrPreviewOutdated) {
		t.Fatalf("stale decision error = %v", err)
	}

	contacts, decisions, err := PrepareImport(data, preview, []models.ImportDecision{{
		Phone: "+79993333333", Action: models.ConflictActionMerge, Version: "2026-07-16T08:30:00Z",
	}})
	if err != nil {
		t.Fatalf("PrepareImport() error = %v", err)
	}
	if len(contacts) != 1 || decisions["+79993333333"].Action != models.ConflictActionMerge {
		t.Fatalf("unexpected prepared import: contacts=%+v decisions=%+v", contacts, decisions)
	}
}

func validDraft() models.ImportDraft {
	return models.ImportDraft{
		ImportID:         testImportID,
		OriginalFilename: "contacts.csv",
		Size:             128,
		Format:           "csv",
		Encoding:         "UTF-8",
		HeaderRow:        1,
		Headers:          []string{"Телефон", "ФИО", "Email"},
		Rows: []models.ImportRow{{
			RowNumber: 2,
			Values: map[string]string{
				"Телефон": "+79991234567",
				"ФИО":     "Иван Иванов",
				"Email":   "user@example.com",
			},
		}},
	}
}
