package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task1/models"
)

type notificationStoreStub struct {
	contacts []models.Contact
	err      error
	calls    int
}

func (s *notificationStoreStub) ListContacts(context.Context) ([]models.Contact, error) {
	s.calls++
	return s.contacts, s.err
}

// TestNotificationPreviewUsesSavedContacts проверяет рассылку без привязки к fileId.
func TestNotificationPreviewUsesSavedContacts(t *testing.T) {
	store := &notificationStoreStub{contacts: []models.Contact{
		{
			Phone:    "+7 (999) 111-22-33",
			Name:     "Анна",
			Email:    "anna@example.com",
			Discount: "15",
		},
		{
			Phone: "+7 (999) 444-55-66",
			Name:  "Иван",
		},
	}}
	handler := &NotificationHandler{store: store}
	body := bytes.NewBufferString(`{"template":"Здравствуйте, {{Имя}}! Телефон: {{Телефон}}, email: {{Email}}, скидка: {{Скидка}}"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/preview", body)
	response := httptest.NewRecorder()

	handler.Preview(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload PreviewResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if store.calls != 1 || len(payload.Notifications) != 2 {
		t.Fatalf("ListContacts calls = %d, notifications = %d", store.calls, len(payload.Notifications))
	}
	if !strings.Contains(payload.Notifications[0].Text, "Анна") ||
		!strings.Contains(payload.Notifications[0].Text, "anna@example.com") ||
		!strings.Contains(payload.Notifications[0].Text, "15") {
		t.Fatalf("unexpected generated text: %q", payload.Notifications[0].Text)
	}
	if payload.Notifications[1].Row != 2 || payload.Notifications[1].Phone != "+7 (999) 444-55-66" {
		t.Fatalf("unexpected second notification: %#v", payload.Notifications[1])
	}
}

// TestNotificationExportUsesSavedContacts проверяет CSV по тем же контактам из БД.
func TestNotificationExportUsesSavedContacts(t *testing.T) {
	store := &notificationStoreStub{contacts: []models.Contact{{
		Phone: "+7 (999) 111-22-33",
		Name:  "Анна",
	}}}
	handler := &NotificationHandler{store: store}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/export",
		bytes.NewBufferString(`{"template":"Здравствуйте, {{Имя}}"}`),
	)
	response := httptest.NewRecorder()

	handler.Export(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	content := response.Body.Bytes()
	if len(content) < 3 || !bytes.Equal(content[:3], []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("CSV does not start with UTF-8 BOM")
	}
	records, err := csv.NewReader(bytes.NewReader(content[3:])).ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	if len(records) != 2 || records[1][0] != "+7 (999) 111-22-33" || records[1][1] != "Здравствуйте, Анна" {
		t.Fatalf("unexpected CSV records: %#v", records)
	}
}

// TestNotificationPreviewRejectsFileColumns проверяет фиксированный набор плейсхолдеров.
func TestNotificationPreviewRejectsFileColumns(t *testing.T) {
	store := &notificationStoreStub{}
	handler := &NotificationHandler{store: store}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/preview",
		bytes.NewBufferString(`{"template":"Город: {{Город}}"}`),
	)
	response := httptest.NewRecorder()

	handler.Preview(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.calls != 0 {
		t.Fatalf("ListContacts was called %d times for invalid template", store.calls)
	}
}
