package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"task1/models"
	"task1/storage"
)

type contactPageStoreStub struct {
	query         string
	limit         int
	offset        int
	updateUID     string
	updateVersion time.Time
	updateContact models.Contact
	updateResult  models.Contact
	updateErr     error
}

func (stub *contactPageStoreStub) ListContactsPage(_ context.Context, query string, limit, offset int) ([]models.Contact, int64, error) {
	stub.query = query
	stub.limit = limit
	stub.offset = offset
	return []models.Contact{{UID: "uid-26", Phone: "+79990000026"}}, 51, nil
}

func (stub *contactPageStoreStub) UpdateContact(
	_ context.Context,
	uid string,
	version time.Time,
	contact models.Contact,
) (models.Contact, error) {
	stub.updateUID = uid
	stub.updateVersion = version
	stub.updateContact = contact
	return stub.updateResult, stub.updateErr
}

func TestContactListUsesFixedServerPagination(t *testing.T) {
	store := &contactPageStoreStub{}
	mux := http.NewServeMux()
	RegisterContactRoutes(mux, store)

	request := httptest.NewRequest(http.MethodGet, "/api/contacts?page=2&q=ivan", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("contact list status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.limit != 25 || store.offset != 25 {
		t.Fatalf("pagination = limit %d offset %d", store.limit, store.offset)
	}
	if store.query != "ivan" {
		t.Fatalf("query = %q", store.query)
	}

	var page models.ContactPage
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Page != 2 || page.PageSize != 25 || page.Total != 51 || page.TotalPages != 3 || page.Query != "ivan" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestContactListRejectsInvalidPage(t *testing.T) {
	mux := http.NewServeMux()
	RegisterContactRoutes(mux, &contactPageStoreStub{})

	request := httptest.NewRequest(http.MethodGet, "/api/contacts?page=0", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("contact list status = %d", response.Code)
	}
}

func TestContactUpdateNormalizesAndSavesFields(t *testing.T) {
	version := "2026-07-16T08:30:00.123456Z"
	uid := "2f656bc0-6227-49d3-9d09-b2d59bd21c52"
	store := &contactPageStoreStub{updateResult: models.Contact{UID: uid, Phone: "+79991234567"}}
	mux := http.NewServeMux()
	RegisterContactRoutes(mux, store)

	body := `{"phone":"8 (999) 123-45-67","email":" USER@EXAMPLE.COM ","name":" Иванов Иван ","discount":"10,5%","version":"` + version + `"}`
	request := httptest.NewRequest(http.MethodPut, "/api/contacts/"+uid, strings.NewReader(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("contact update status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.updateUID != uid {
		t.Fatalf("update uid = %q", store.updateUID)
	}
	if store.updateContact.Phone != "+79991234567" || store.updateContact.Email != "user@example.com" ||
		store.updateContact.Name != "Иванов Иван" || store.updateContact.Discount != "10.5" {
		t.Fatalf("unexpected normalized contact: %+v", store.updateContact)
	}
	wantVersion, _ := time.Parse(time.RFC3339Nano, version)
	if !store.updateVersion.Equal(wantVersion) {
		t.Fatalf("update version = %s", store.updateVersion)
	}
}

func TestContactUpdateReturnsConflict(t *testing.T) {
	uid := "2f656bc0-6227-49d3-9d09-b2d59bd21c52"
	store := &contactPageStoreStub{updateErr: storage.ErrContactPhoneExists}
	mux := http.NewServeMux()
	RegisterContactRoutes(mux, store)

	body := `{"phone":"+79991234567","version":"2026-07-16T08:30:00Z"}`
	request := httptest.NewRequest(http.MethodPut, "/api/contacts/"+uid, strings.NewReader(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("contact update status = %d, body = %s", response.Code, response.Body.String())
	}
	var apiError ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&apiError); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !errors.Is(store.updateErr, storage.ErrContactPhoneExists) || apiError.Error != storage.ErrContactPhoneExists.Error() {
		t.Fatalf("unexpected conflict: %+v", apiError)
	}
}
