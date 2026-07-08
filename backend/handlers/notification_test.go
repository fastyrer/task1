package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task1/backend/models"
	"task1/backend/storage"
)

func setupNotificationTest(t *testing.T) (*NotificationHandler, string) {
	t.Helper()

	store := storage.NewMemoryStorage()
	handler := &NotificationHandler{store: store}

	data := models.FileData{
		Headers: []string{"Телефон", "Имя", "Скидка"},
		Rows: []map[string]string{
			{"Телефон": "+79990001122", "Имя": "Анна", "Скидка": "15"},
			{"Телефон": "+79990001133", "Имя": "Иван", "Скидка": "10"},
			{"Телефон": "", "Имя": "Олег", "Скидка": "5"},
		},
	}

	fileID := store.SaveFileData(data)
	return handler, fileID
}

func TestPreview_Success(t *testing.T) {
	handler, fileID := setupNotificationTest(t)

	body := previewRequest{
		FileID:      fileID,
		PhoneColumn: "Телефон",
		Template:    "Привет, {{Имя}}! Скидка: {{Скидка}}%",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Preview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получил %d: %s", w.Code, w.Body.String())
	}

	var resp previewResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Notifications) != 2 {
		t.Fatalf("ожидалось 2 уведомления (строка без телефона пропущена), получил %d", len(resp.Notifications))
	}

	if resp.Skipped != 1 {
		t.Fatalf("ожидался 1 пропуск, получил %d", resp.Skipped)
	}

	if resp.Notifications[0].Phone != "+79990001122" {
		t.Fatalf("неверный телефон: %s", resp.Notifications[0].Phone)
	}

	if resp.Notifications[0].Text != "Привет, Анна! Скидка: 15%" {
		t.Fatalf("неверный текст: %s", resp.Notifications[0].Text)
	}
}

func TestPreview_InvalidFileID(t *testing.T) {
	handler, _ := setupNotificationTest(t)

	body := previewRequest{
		FileID:      "nonexistent",
		PhoneColumn: "Телефон",
		Template:    "Привет, {{Имя}}!",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Preview(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидался 404, получил %d", w.Code)
	}
}

func TestPreview_UnknownPlaceholder(t *testing.T) {
	handler, fileID := setupNotificationTest(t)

	body := previewRequest{
		FileID:      fileID,
		PhoneColumn: "Телефон",
		Template:    "Привет, {{Неизвестный}}!",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Preview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получил %d", w.Code)
	}

	var errResp errorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Error == "" {
		t.Fatal("ожидалось сообщение об ошибке")
	}
}

func TestPreview_EmptyTemplate(t *testing.T) {
	handler, fileID := setupNotificationTest(t)

	body := previewRequest{
		FileID:      fileID,
		PhoneColumn: "Телефон",
		Template:    "",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Preview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получил %d", w.Code)
	}
}

func TestPreview_PhoneColumnNotFound(t *testing.T) {
	handler, fileID := setupNotificationTest(t)

	body := previewRequest{
		FileID:      fileID,
		PhoneColumn: "Адрес",
		Template:    "Привет!",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Preview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получил %d", w.Code)
	}
}

func TestExport_Success(t *testing.T) {
	handler, fileID := setupNotificationTest(t)

	body := previewRequest{
		FileID:      fileID,
		PhoneColumn: "Телефон",
		Template:    "Привет, {{Имя}}!",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest(http.MethodPost, "/api/export", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получил %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv; charset=utf-8" {
		t.Fatalf("неверный Content-Type: %s", contentType)
	}

	disposition := w.Header().Get("Content-Disposition")
	if disposition != "attachment; filename=notifications.csv" {
		t.Fatalf("неверный Content-Disposition: %s", disposition)
	}

	bodyStr := w.Body.String()
	if !bytes.Contains([]byte(bodyStr), []byte("+79990001122")) {
		t.Fatal("CSV не содержит номер телефона")
	}
	if bytes.Contains([]byte(bodyStr), []byte("Олег")) {
		t.Fatal("CSV содержит строку с пустым телефоном")
	}
}
