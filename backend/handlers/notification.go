// Package handlers реализует методы для приема и обработки файлов

package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"task1/backend/services"
	"task1/backend/storage"
)

// NotificationHandler инкапсулирует доступ к хранилищу файлов
type NotificationHandler struct {
	store *storage.MemoryStorage
}

// previewRequest 
type previewRequest struct {
	FileID      string `json:"fileId"`
	PhoneColumn string `json:"phoneColumn"`
	Template    string `json:"template"`
}

// notificationItem
type notificationItem struct {
	Phone string `json:"phone"`
	Text  string `json:"text"`
	Row   int    `json:"row"`
}

// previewResponse 
type previewResponse struct {
	Notifications []notificationItem `json:"notifications"`
	Skipped       int                `json:"skipped"`
}

func RegisterNotificationRoutes(mux *http.ServeMux, store *storage.MemoryStorage) {
	h := &NotificationHandler{store: store}
	mux.HandleFunc("/api/preview", h.Preview)
	mux.HandleFunc("/api/export", h.Export)
}

func (h *NotificationHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	_, ok := h.store.GetFileData(req.FileID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден. Загрузите файл снова.")
		return
	}

	resp, err := h.generate(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	_, ok := h.store.GetFileData(req.FileID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден. Загрузите файл снова.")
		return
	}

	resp, err := h.generate(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xef, 0xbb, 0xbf})

	writer := csv.NewWriter(&buf)
	writer.Write([]string{"Телефон", "Сообщение"})
	for _, n := range resp.Notifications {
		writer.Write([]string{n.Phone, n.Text})
	}
	writer.Flush()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=notifications.csv")
	w.Write(buf.Bytes())
}

func (h *NotificationHandler) generate(req previewRequest) (previewResponse, error) {
	data, _ := h.store.GetFileData(req.FileID)

	phoneExists := false
	for _, h := range data.Headers {
		if h == req.PhoneColumn {
			phoneExists = true
			break
		}
	}
	if !phoneExists {
		return previewResponse{}, fmt.Errorf("Колонка '%s' не найдена в файле.", req.PhoneColumn)
	}

	if strings.TrimSpace(req.Template) == "" {
		return previewResponse{}, services.ErrEmptyTemplate
	}

	placeholders := services.ParsePlaceholders(req.Template)
	if err := services.ValidateUnknownPlaceholders(placeholders, data.Headers); err != nil {
		return previewResponse{}, err
	}

	invalidSet := make(map[int]struct{}, len(data.InvalidRows))
	for _, inv := range data.InvalidRows {
		invalidSet[inv.Row] = struct{}{}
	}

	notifications := make([]notificationItem, 0, len(data.Rows))
	skipped := 0

	for i, row := range data.Rows {
		phone := strings.TrimSpace(row[req.PhoneColumn])
		if phone == "" {
			skipped++
			continue
		}

		if i < len(data.RowNumbers) {
			if _, ok := invalidSet[data.RowNumbers[i]]; ok {
				skipped++
				continue
			}
		}

		text := services.GenerateText(req.Template, row)
		notifications = append(notifications, notificationItem{
			Phone: phone,
			Text:  text,
			Row:   i + 1,
		})
	}

	return previewResponse{
		Notifications: notifications,
		Skipped:       skipped,
	}, nil
}
