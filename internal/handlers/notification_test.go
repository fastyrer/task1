package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task1/internal/models"
)

type notificationStoreStub struct {
	contacts []models.Contact
}

func (stub notificationStoreStub) ListContactsPage(
	_ context.Context,
	_ string,
	limit int,
	offset int,
) ([]models.Contact, int64, error) {
	if offset >= len(stub.contacts) {
		return []models.Contact{}, int64(len(stub.contacts)), nil
	}
	end := min(offset+limit, len(stub.contacts))
	return stub.contacts[offset:end], int64(len(stub.contacts)), nil
}

func (stub notificationStoreStub) VisitContacts(_ context.Context, visit func(models.Contact) error) error {
	for _, contact := range stub.contacts {
		if err := visit(contact); err != nil {
			return err
		}
	}
	return nil
}

func TestNotificationPreviewRejectsUnknownFields(t *testing.T) {
	mux := http.NewServeMux()
	RegisterNotificationRoutes(mux, notificationStoreStub{})

	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/preview",
		strings.NewReader(`{"template":"Здравствуйте","unexpected":true}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestNotificationPreviewRejectsOversizedBody(t *testing.T) {
	mux := http.NewServeMux()
	RegisterNotificationRoutes(mux, notificationStoreStub{})

	body := `{"template":"` + strings.Repeat("a", maxNotificationRequestBody) + `"}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/preview", strings.NewReader(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestGenerateNotificationsLimitsPreview(t *testing.T) {
	contacts := []models.Contact{
		{Phone: "+79990000001", Name: "Первый"},
		{Phone: ""},
		{Phone: "+79990000002", Name: "Второй"},
		{Phone: "+79990000003", Name: "Третий"},
	}

	result := generateNotifications(contacts, "Здравствуйте, {{Имя}}", 2)
	if result.Total != 3 || result.Returned != 2 || result.Skipped != 1 || !result.Truncated {
		t.Fatalf("unexpected preview: %+v", result)
	}
	if len(result.Notifications) != 2 || result.Notifications[1].Row != 3 {
		t.Fatalf("unexpected notifications: %+v", result.Notifications)
	}
}

func TestNotificationExportIncludesAllContacts(t *testing.T) {
	store := notificationStoreStub{contacts: []models.Contact{
		{Phone: "+79990000001", Name: "Первый"},
		{Phone: "+79990000002", Name: "Второй"},
	}}
	mux := http.NewServeMux()
	RegisterNotificationRoutes(mux, store)

	body, err := json.Marshal(PreviewRequest{Template: "Здравствуйте, {{Имя}}"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/export", strings.NewReader(string(body)))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{"+79990000001", "+79990000002"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("export does not contain %q: %s", expected, response.Body.String())
		}
	}
}
