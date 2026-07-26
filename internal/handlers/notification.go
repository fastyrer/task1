package handlers

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"task1/internal/models"
	"task1/internal/services"
)

const (
	phonePlaceholder    = "Телефон"
	namePlaceholder     = "Имя"
	emailPlaceholder    = "Email"
	discountPlaceholder = "Скидка"

	maxNotificationRequestBody = 64 << 10
	maxNotificationTemplate    = 10_000
	defaultNotificationPreview = 500
	maxNotificationPreview     = 5_000
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
	ListContactsPage(ctx context.Context, query string, limit, offset int) ([]models.Contact, int64, error)
	VisitContacts(ctx context.Context, visit func(models.Contact) error) error
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
	Total         int64              `json:"total" example:"1200"`
	Returned      int                `json:"returned" example:"500"`
	Truncated     bool               `json:"truncated" example:"true"`
}

// RegisterNotificationRoutes привязывает два эндпоинта к одному NotificationHandler.
func RegisterNotificationRoutes(mux *http.ServeMux, store NotificationStore) {
	h := &NotificationHandler{store: store}
	mux.HandleFunc("/api/preview", h.Preview)
	mux.HandleFunc("/api/export", h.Export)
}

// Preview формирует JSON-предпросмотр по всем актуальным контактам.
// @Summary Предпросмотр общей рассылки
// @Description Формирует ограниченный предпросмотр сообщений по актуальным контактам PostgreSQL. CSV-экспорт всегда включает весь справочник. Поддерживаются плейсхолдеры {{Телефон}}, {{Имя}}, {{Email}} и {{Скидка}}.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param request body PreviewRequest true "Шаблон сообщения"
// @Success 200 {object} PreviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
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
	if !decodeJSONBody(w, r, &req, maxNotificationRequestBody) {
		return
	}
	if err := validateNotificationTemplate(req.Template); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := notificationPreviewLimit()
	contacts, total, err := h.store.ListContactsPage(r.Context(), "", limit, 0)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}

	response := generateNotifications(contacts, req.Template, limit)
	response.Total = total
	response.Truncated = total > int64(response.Returned)
	writeJSON(w, http.StatusOK, response)
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
// @Failure 413 {object} ErrorResponse
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
	if !decodeJSONBody(w, r, &req, maxNotificationRequestBody) {
		return
	}
	if err := validateNotificationTemplate(req.Template); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var writer *csv.Writer
	started := false
	startCSV := func() error {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=notifications.csv")
		started = true
		if _, err := w.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
			return err
		}
		writer = csv.NewWriter(w)
		return writer.Write([]string{"Телефон", "Сообщение"})
	}

	row := 0
	err := h.store.VisitContacts(r.Context(), func(contact models.Contact) error {
		row++
		notification, ok := notificationFromContact(contact, req.Template, row)
		if !ok {
			return nil
		}
		if !started {
			if err := startCSV(); err != nil {
				return err
			}
		}
		return writer.Write([]string{notification.Phone, notification.Text})
	})
	if err != nil {
		if !started {
			writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		}
		return
	}
	if !started {
		if err := startCSV(); err != nil {
			return
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		return
	}
}

// validateNotificationTemplate проверяет шаблон до обращения к PostgreSQL.
func validateNotificationTemplate(template string) error {
	if strings.TrimSpace(template) == "" {
		return errors.New(services.ErrorTemplateEmpty)
	}
	if utf8.RuneCountInString(template) > maxNotificationTemplate {
		return errors.New(services.ErrorTemplateTooLong)
	}

	placeholders := services.ParsePlaceholders(template)
	return services.ValidateUnknownPlaceholders(placeholders, contactTemplateFields)
}

// generateNotifications формирует ограниченный HTTP-предпросмотр, но считает
// полный объём рассылки. CSV-export обрабатывает весь справочник потоково.
func generateNotifications(contacts []models.Contact, template string, limit int) PreviewResponse {
	if limit <= 0 {
		limit = defaultNotificationPreview
	}
	notifications := make([]NotificationItem, 0, min(limit, len(contacts)))
	skipped := 0
	var total int64

	for index, contact := range contacts {
		notification, ok := notificationFromContact(contact, template, index+1)
		if !ok {
			skipped++
			continue
		}
		total++
		if len(notifications) < limit {
			notifications = append(notifications, notification)
		}
	}

	return PreviewResponse{
		Notifications: notifications,
		Skipped:       skipped,
		Total:         total,
		Returned:      len(notifications),
		Truncated:     total > int64(len(notifications)),
	}
}

func notificationFromContact(contact models.Contact, template string, row int) (NotificationItem, bool) {
	phone := strings.TrimSpace(contact.Phone)
	if phone == "" {
		return NotificationItem{}, false
	}

	values := map[string]string{
		phonePlaceholder:    phone,
		namePlaceholder:     contact.Name,
		emailPlaceholder:    contact.Email,
		discountPlaceholder: contact.Discount,
	}
	return NotificationItem{
		Phone: phone,
		Text:  services.GenerateText(template, values),
		Row:   row,
	}, true
}

func notificationPreviewLimit() int {
	value := strings.TrimSpace(os.Getenv("NOTIFICATION_PREVIEW_LIMIT"))
	if value == "" {
		return defaultNotificationPreview
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 || parsed > maxNotificationPreview {
		return defaultNotificationPreview
	}
	return parsed
}
