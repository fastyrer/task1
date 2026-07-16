// contact.go - поиск, пагинация и ручное редактирование справочника PostgreSQL.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"task1/models"
	"task1/services"
	"task1/storage"
	"task1/utils"
)

const (
	contactPageSize       = 25
	maxContactPage        = 1_000_000
	maxContactSearchRunes = 200
	maxContactUpdateBody  = 64 << 10
)

// ContactHandler получает только операции, необходимые экрану справочника.
type ContactHandler struct {
	store storage.ContactDirectoryStore
}

// RegisterContactRoutes регистрирует просмотр, поиск и изменение одного контакта.
func RegisterContactRoutes(mux *http.ServeMux, store storage.ContactDirectoryStore) {
	handler := &ContactHandler{store: store}
	mux.HandleFunc("/api/contacts", handler.List)
	mux.HandleFunc("/api/contacts/", handler.Update)
}

// List возвращает одну страницу контактов; размер страницы зафиксирован в 25.
// @Summary Получить страницу контактов
// @Description Возвращает 25 подтверждённых контактов PostgreSQL и общее количество совпадений. Поиск выполняется по телефону, ФИО, email, скидке и UID.
// @Tags Contacts
// @Produce json
// @Param page query int false "Номер страницы, начиная с 1" default(1)
// @Param q query string false "Подстрока для поиска по всем видимым полям"
// @Success 200 {object} models.ContactPage
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/contacts [get]
func (h *ContactHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	page, ok := contactPageNumber(r)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}
	query, ok := contactSearchQuery(r)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, services.ErrorContactQueryTooLong)
		return
	}

	items, total, err := h.store.ListContactsPage(r.Context(), query, contactPageSize, (page-1)*contactPageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + contactPageSize - 1) / contactPageSize)
	}
	writeJSON(w, http.StatusOK, models.ContactPage{
		Items:      items,
		Page:       page,
		PageSize:   contactPageSize,
		Total:      total,
		TotalPages: totalPages,
		Query:      query,
	})
}

// Update изменяет одну актуальную запись справочника после проверки её версии.
// @Summary Изменить контакт
// @Description Нормализует поля и обновляет контакт, только если его updatedAt не изменился. Телефон остаётся уникальным.
// @Tags Contacts
// @Accept json
// @Produce json
// @Param uid path string true "UUID контакта"
// @Param request body models.ContactUpdateRequest true "Изменяемые поля и текущая версия"
// @Success 200 {object} models.Contact
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/contacts/{uid} [put]
func (h *ContactHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	uid := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/contacts/"))
	if strings.Contains(uid, "/") || !utils.IsUUID(uid) {
		writeJSONError(w, http.StatusBadRequest, services.ErrorContactUIDInvalid)
		return
	}

	var request models.ContactUpdateRequest
	if !decodeContactUpdate(w, r, &request) {
		return
	}
	contact, version, err := services.ValidateContactUpdate(request)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	updated, err := h.store.UpdateContact(r.Context(), uid, version, contact)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrContactChanged), errors.Is(err, storage.ErrContactPhoneExists):
			writeJSONError(w, http.StatusConflict, err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, services.ErrorContactNotUpdated)
		}
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func contactPageNumber(r *http.Request) (int, bool) {
	value := r.URL.Query().Get("page")
	if value == "" {
		return 1, true
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > maxContactPage {
		return 0, false
	}
	return page, true
}

func contactSearchQuery(r *http.Request) (string, bool) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	return query, utf8.RuneCountInString(query) <= maxContactSearchRunes
}

func decodeContactUpdate(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxContactUpdateBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return false
	}
	return true
}
