// Package handlers содержит HTTP-обработчики для всех эндпоинтов приложения.
//
// notification.go - генерация уведомлений по шаблону.
// POST /api/preview формирует уведомления из актуальных контактов PostgreSQL
// с фиксированными плейсхолдерами и возвращает JSON с предпросмотром.
// POST /api/export делает то же самое, но возвращает CSV-файл
// с колонками Телефон,Сообщение для скачивания.

package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"task1/models"
	"task1/services"
)

const (
	phonePlaceholder    = "Телефон"
	namePlaceholder     = "Имя"
	emailPlaceholder    = "Email"
	discountPlaceholder = "Скидка"
)

// contactTemplateFields - полный список полей, доступных в шаблоне рассылки.
var contactTemplateFields = []string{
	phonePlaceholder,
	namePlaceholder,
	emailPlaceholder,
	discountPlaceholder,
}

// NotificationStore ограничивает обработчик единственной нужной операцией с контактами.
type NotificationStore interface {
	ListContacts(ctx context.Context) ([]models.Contact, error)
}

// NotificationHandler инкапсулирует чтение актуальных контактов из PostgreSQL.
type NotificationHandler struct {
	store NotificationStore
}

// PreviewRequest содержит шаблон общей рассылки по сохранённым контактам.
type PreviewRequest struct {
	Template string `json:"template" validate:"required" example:"Здравствуйте, {{Имя}}! Ваша скидка: {{Скидка}}."`
}

// NotificationItem – готовое уведомление для одного контакта.
type NotificationItem struct {
	Phone string `json:"phone" example:"+79991234567"`
	Text  string `json:"text" example:"Здравствуйте, Анна! Ваша скидка: 10%."`
	Row   int    `json:"row" example:"1"`
}

// PreviewResponse содержит готовые уведомления и число пропущенных контактов.
type PreviewResponse struct {
	Notifications []NotificationItem `json:"notifications"`
	Skipped       int                `json:"skipped" example:"0"`
}

// RegisterNotificationRoutes привязывает два эндпоинта к одному NotificationHandler.
func RegisterNotificationRoutes(mux *http.ServeMux, store NotificationStore) {
	h := &NotificationHandler{store: store}
	mux.HandleFunc("/api/preview", h.Preview)
	mux.HandleFunc("/api/export", h.Export)
}

// Preview формирует JSON-предпросмотр по всем актуальным контактам.
// @Summary Предпросмотр общей рассылки
// @Description Формирует сообщение для каждого актуального контакта PostgreSQL. Поддерживаются плейсхолдеры {{Телефон}}, {{Имя}}, {{Email}} и {{Скидка}}.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param request body PreviewRequest true "Шаблон сообщения"
// @Success 200 {object} PreviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/preview [post]
func (h *NotificationHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}
	if err := validateNotificationTemplate(req.Template); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	contacts, err := h.store.ListContacts(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}

	writeJSON(w, http.StatusOK, generateNotifications(contacts, req.Template))
}

// Export формирует те же уведомления и возвращает CSV-файл с UTF-8 BOM.
// @Summary Экспортировать общую рассылку
// @Description Формирует сообщения по всем актуальным контактам PostgreSQL и возвращает UTF-8 CSV с колонками «Телефон» и «Сообщение».
// @Tags Notifications
// @Accept json
// @Produce text/csv,application/json
// @Param request body PreviewRequest true "Шаблон сообщения"
// @Success 200 {file} file "notifications.csv"
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/export [post]
func (h *NotificationHandler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}
	if err := validateNotificationTemplate(req.Template); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	contacts, err := h.store.ListContacts(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}
	resp := generateNotifications(contacts, req.Template)

	var buf bytes.Buffer
	buf.Write([]byte{0xef, 0xbb, 0xbf})

	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"Телефон", "Сообщение"}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorCSVNotCreated)
		return
	}
	for _, notification := range resp.Notifications {
		if err := writer.Write([]string{notification.Phone, notification.Text}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, services.ErrorCSVNotCreated)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorCSVNotCreated)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=notifications.csv")
	w.Write(buf.Bytes())
}

// validateNotificationTemplate проверяет шаблон до обращения к PostgreSQL.
func validateNotificationTemplate(template string) error {
	if strings.TrimSpace(template) == "" {
		return errors.New(services.ErrorTemplateEmpty)
	}

	placeholders := services.ParsePlaceholders(template)
	return services.ValidateUnknownPlaceholders(placeholders, contactTemplateFields)
}

// generateNotifications формирует по одному сообщению на уникальный телефон из contacts.
func generateNotifications(contacts []models.Contact, template string) PreviewResponse {
	notifications := make([]NotificationItem, 0, len(contacts))
	skipped := 0

	for index, contact := range contacts {
		phone := strings.TrimSpace(contact.Phone)
		if phone == "" {
			skipped++
			continue
		}

		values := map[string]string{
			phonePlaceholder:    phone,
			namePlaceholder:     contact.Name,
			emailPlaceholder:    contact.Email,
			discountPlaceholder: contact.Discount,
		}
		notifications = append(notifications, NotificationItem{
			Phone: phone,
			Text:  services.GenerateText(template, values),
			Row:   index + 1,
		})
	}

	return PreviewResponse{
		Notifications: notifications,
		Skipped:       skipped,
	}
}
