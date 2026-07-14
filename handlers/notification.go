// Package handlers содержит HTTP-обработчики для всех эндпоинтов приложения.
//
// notification.go – генерация уведомлений по шаблону.
// POST /api/preview формирует уведомления из строк файла по шаблону
// с плейсхолдерами {{Name}} и возвращает JSON с предпросмотром.
// POST /api/export делает то же самое, но возвращает CSV-файл
// с колонками Телефон,Сообщение для скачивания.

package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"task1/models"
	"task1/services"
	"task1/storage"
)

// NotificationHandler инкапсулирует доступ к хранилищу файлов
type NotificationHandler struct {
	store storage.FileStore
}

// previewRequest содержит выбранный файл, телефонную колонку и шаблон.
type previewRequest struct {
	FileID      string `json:"fileId"`
	PhoneColumn string `json:"phoneColumn"`
	Template    string `json:"template"`
}

// notificationItem - готовое уведомление для одной строки файла.
type notificationItem struct {
	Phone string `json:"phone"`
	Text  string `json:"text"`
	Row   int    `json:"row"`
}

// previewResponse содержит готовые уведомления и число пропущенных строк.
type previewResponse struct {
	Notifications []notificationItem `json:"notifications"`
	Skipped       int                `json:"skipped"`
}

// Привязывает два эндпоинта к одному NotificationHandler.
func RegisterNotificationRoutes(mux *http.ServeMux, store storage.FileStore) {
	h := &NotificationHandler{store: store}
	mux.HandleFunc("/api/preview", h.Preview)
	mux.HandleFunc("/api/export", h.Export)
}

// Preview формирует JSON-предпросмотр уведомлений для выбранного файла.
func (h *NotificationHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}

	data, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	resp, err := h.generate(data, req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Export формирует те же уведомления и возвращает CSV-файл с UTF-8 BOM.
func (h *NotificationHandler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}

	data, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	resp, err := h.generate(data, req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xef, 0xbb, 0xbf})

	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"Телефон", "Сообщение"}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось сформировать CSV")
		return
	}
	for _, n := range resp.Notifications {
		if err := writer.Write([]string{n.Phone, n.Text}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Не удалось сформировать CSV")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось сформировать CSV")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=notifications.csv")
	w.Write(buf.Bytes())
}

// generate проверяет шаблон и формирует уведомления только из валидных строк файла.
func (h *NotificationHandler) generate(data models.FileData, req previewRequest) (previewResponse, error) {
	req.PhoneColumn = strings.TrimSpace(req.PhoneColumn)
	phoneExists := false
	for _, h := range data.Headers {
		if h == req.PhoneColumn {
			phoneExists = true
			break
		}
	}
	if !phoneExists {
		return previewResponse{}, fmt.Errorf("колонка %q не найдена в файле", req.PhoneColumn)
	}

	if strings.TrimSpace(req.Template) == "" {
		return previewResponse{}, fmt.Errorf("%s", services.ErrorTemplateEmpty)
	}

	placeholders := services.ParsePlaceholders(req.Template)
	if err := services.ValidateUnknownPlaceholders(placeholders, data.Headers); err != nil {
		return previewResponse{}, err
	}

	invalidSet := invalidRowNumbers(data.InvalidRows)

	notifications := make([]notificationItem, 0, len(data.Rows))
	skipped := 0

	for i, row := range data.Rows {
		phone := strings.TrimSpace(row[req.PhoneColumn])
		if phone == "" {
			skipped++
			continue
		}

		rowNumber := fileRowNumber(data, i)
		if _, invalid := invalidSet[rowNumber]; invalid {
			skipped++
			continue
		}

		text := services.GenerateText(req.Template, row)
		notifications = append(notifications, notificationItem{
			Phone: phone,
			Text:  text,
			Row:   rowNumber,
		})
	}

	return previewResponse{
		Notifications: notifications,
		Skipped:       skipped,
	}, nil
}
