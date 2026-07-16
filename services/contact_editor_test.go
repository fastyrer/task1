package services

import (
	"errors"
	"testing"

	"task1/models"
)

func TestValidateContactUpdateAllowsEmptyOptionalFields(t *testing.T) {
	contact, version, err := ValidateContactUpdate(models.ContactUpdateRequest{
		Phone:   "+7 999 123-45-67",
		Version: "2026-07-16T08:30:00.123456Z",
	})
	if err != nil {
		t.Fatalf("validate contact update: %v", err)
	}
	if contact.Phone != "+79991234567" || contact.Email != "" || contact.Name != "" || contact.Discount != "" {
		t.Fatalf("unexpected contact: %+v", contact)
	}
	if version.IsZero() {
		t.Fatal("version must be parsed")
	}
}

func TestValidateContactUpdateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		request models.ContactUpdateRequest
		wantErr error
	}{
		{
			name:    "phone",
			request: models.ContactUpdateRequest{Phone: "123", Version: "2026-07-16T08:30:00Z"},
			wantErr: ErrContactPhoneInvalid,
		},
		{
			name:    "email",
			request: models.ContactUpdateRequest{Phone: "+79991234567", Email: "wrong", Version: "2026-07-16T08:30:00Z"},
			wantErr: ErrContactEmailInvalid,
		},
		{
			name:    "discount",
			request: models.ContactUpdateRequest{Phone: "+79991234567", Discount: "101", Version: "2026-07-16T08:30:00Z"},
			wantErr: ErrContactDiscountInvalid,
		},
		{
			name:    "version",
			request: models.ContactUpdateRequest{Phone: "+79991234567"},
			wantErr: ErrContactVersionInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := ValidateContactUpdate(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
