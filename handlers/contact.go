// Package handlers содержит HTTP-обработчики для всех эндпоинтов приложения.
//
// contact.go – управление контактами: сохранение строк из файла как контактов,
// разрешение конфликтов (когда контакт с таким телефоном уже существует),
// и исправление невалидных строк через редактирование на фронте.

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"task1/models"
	"task1/services"
	"task1/storage"
	"task1/utils"
)

// ContactHandler – обработчик для работы с контактами
type ContactHandler struct {
	store    storage.FileStore
	contacts storage.ContactStore
}

// RegisterContactRoutes – регистрация маршрутов для работы с контактами
func RegisterContactRoutes(mux *http.ServeMux, store storage.FileStore, contacts storage.ContactStore) {
	h := &ContactHandler{store: store, contacts: contacts}
	mux.HandleFunc("/api/contacts/save", h.Save)
	mux.HandleFunc("/api/contacts/resolve", h.Resolve)
	mux.HandleFunc("/api/contacts/resolve-all", h.ResolveAll)
	mux.HandleFunc("/api/rows/fix", h.FixRows)
}

// FixRowsRequest – запрос на проверку и сохранение исправленных строк.
type FixRowsRequest struct {
	FileID string                 `json:"fileId" validate:"required" example:"2f656bc0-6227-49d3-9d09-b2d59bd21c52"`
	Rows   []services.FixRowInput `json:"rows" validate:"required"`
}

// FixRows – POST /api/rows/fix
//
// Позволяет пользователю отредактировать невалидные строки на фронте
// (в таблице "Строки с ошибками") и отправить исправления на сервер.
//
// Алгоритм:
//  1. Получаем данные файла.
//  2. Определяем колонку телефона (обязательна для контакта).
//  3. Для каждой исправленной строки:
//     – Нормализуем телефон/email/скидку
//     – Сохраняем контакт в ContactStore
//     – Если нормализация не удалась – добавляем в список Failed
//  4. Возвращаем {fixed: N, failed: [{row, errors}]}
//
// @Summary Проверить и сохранить исправленные строки
// @Description Повторно нормализует отредактированные пользователем строки файла и сохраняет корректные контакты в PostgreSQL.
// @Tags Files
// @Accept json
// @Produce json
// @Param request body FixRowsRequest true "Исправленные строки"
// @Success 200 {object} services.FixRowResult
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/rows/fix [post]
func (h *ContactHandler) FixRows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req FixRowsRequest
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

	phoneColumn := utils.DetectPhoneColumn(data.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
		return
	}

	result := services.FixRowResult{}
	for _, row := range req.Rows {
		if err := services.FixAndSaveRow(r.Context(), h.contacts, row, phoneColumn, data.ID); err != nil {
			result.Failed = append(result.Failed, *err)
		} else {
			result.Fixed++
		}
	}
	if refreshed, found, refreshErr := h.store.GetFileData(r.Context(), data.ID); refreshErr == nil && found {
		result.Stats = &refreshed.Stats
		result.Warnings = refreshed.Warnings
	}

	writeJSON(w, http.StatusOK, result)
}

// SaveContactsRequest – запрос на сохранение валидных строк файла как контактов.
type SaveContactsRequest struct {
	FileID string `json:"fileId" validate:"required" example:"2f656bc0-6227-49d3-9d09-b2d59bd21c52"`
}

// SaveContactsResponse – результат сохранения и найденные конфликты.
type SaveContactsResponse struct {
	Saved     int                   `json:"saved"`               // Сколько сохранено
	Skipped   int                   `json:"skipped"`             // Сколько пропущено
	Conflicts []models.ConflictInfo `json:"conflicts,omitempty"` // конфликты
	// с существующими контактами
}

// ResolveResponse – результат применения решения к одному конфликту.
type ResolveResponse struct {
	Status string                `json:"status" enums:"ok" example:"ok"`
	Phone  string                `json:"phone" example:"+79991234567"`
	Action models.ConflictAction `json:"action" enums:"skip,replace,merge" example:"merge"`
}

// ResolveAllResponse – результат применения одного решения ко всем конфликтам файла.
type ResolveAllResponse struct {
	Status   string                `json:"status" enums:"ok" example:"ok"`
	Resolved int                   `json:"resolved" example:"3"`
	Action   models.ConflictAction `json:"action" enums:"skip,replace,merge" example:"merge"`
}

// Save – POST /api/contacts/save
//
// Проходит по всем валидным строкам файла и сохраняет их как контакты.
// Если контакт с таким телефоном уже существует и данные различаются –
// не сохраняет, а возвращает ConflictInfo с описанием расхождений.
//
// Фронт получает список конфликтов и предлагает пользователю выбрать действие:
// skip (пропустить), replace (заменить) или merge (слить).
// @Summary Сохранить контакты из файла
// @Description Переносит валидные строки загруженного файла в общий справочник контактов. Несовпадающие данные для существующего телефона возвращаются как конфликты.
// @Tags Contacts
// @Accept json
// @Produce json
// @Param request body SaveContactsRequest true "Идентификатор загруженного файла"
// @Success 200 {object} SaveContactsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/contacts/save [post]
func (h *ContactHandler) Save(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req SaveContactsRequest
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

	phoneColumn := utils.DetectPhoneColumn(data.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
		return
	}

	result, err := services.ProcessContacts(r.Context(), h.contacts, data, phoneColumn)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SaveContactsResponse{
		Saved:     result.Saved,
		Skipped:   result.Skipped,
		Conflicts: result.Conflicts,
	})
}

// Resolve – POST /api/contacts/resolve
//
// Разрешает один конфликт по номеру телефона.
// Frontend присылает: fileId, phone (телефон, по которому конфликт),
// action (skip / replace / merge).
//
// Алгоритм:
//  1. Проверяем, что action допустимый.
//  2. Ищем строку с указанным телефоном в данных файла.
//  3. Преобразуем строку в Contact (RowToContact).
//  4. Вызываем ResolveConflict в хранилище – оно применяет выбранное действие.
//
// @Summary Разрешить один конфликт контакта
// @Description Применяет действие skip, replace или merge к несовпадающим данным одного телефона из выбранного файла.
// @Tags Contacts
// @Accept json
// @Produce json
// @Param request body models.ResolveRequest true "Решение конфликта"
// @Success 200 {object} ResolveResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/contacts/resolve [post]
func (h *ContactHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req models.ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneEmpty)
		return
	}

	switch req.Action {
	case models.ConflictActionSkip, models.ConflictActionReplace, models.ConflictActionMerge:
	default:
		writeJSONError(w, http.StatusBadRequest, services.ErrorUnsupportedAction)
		return
	}

	fd, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
		return
	}

	var incoming models.Contact
	var found bool
	for index, row := range fd.Rows {
		if strings.TrimSpace(row[phoneColumn]) == req.Phone {
			incoming = services.RowToContact(row, req.Phone, fd.ID)
			incoming.SourceRow = fileRowNumber(fd, index)
			found = true
			break
		}
	}

	if !found {
		writeJSONError(w, http.StatusNotFound, services.ErrorPhoneNotFound)
		return
	}

	if err := h.contacts.ResolveConflict(r.Context(), req.Phone, req.Action, incoming); err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorConflictNotSolved)
		return
	}

	writeJSON(w, http.StatusOK, ResolveResponse{
		Status: "ok",
		Phone:  req.Phone,
		Action: req.Action,
	})
}

// ResolveAll – POST /api/contacts/resolve-all
//
// Применяет одно действие (skip/replace/merge) ко всем конфликтам в файле.
// Используется, когда пользователь выбрал "разрешить все" на фронте.
//
// Алгоритм:
//  1. Проходит по всем строкам файла.
//  2. Для каждой строки с непустым телефоном проверяет,
//     есть ли контакт с таким телефоном в хранилище.
//  3. Если есть и контакты различаются – применяет выбранное действие.
//  4. Возвращает количество разрешённых конфликтов.
//
// @Summary Разрешить все конфликты файла
// @Description Применяет одно действие skip, replace или merge ко всем несовпадающим контактам выбранного файла.
// @Tags Contacts
// @Accept json
// @Produce json
// @Param request body models.BatchResolveRequest true "Общее решение конфликтов"
// @Success 200 {object} ResolveAllResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/contacts/resolve-all [post]
func (h *ContactHandler) ResolveAll(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req models.BatchResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}

	switch req.Action {
	case models.ConflictActionSkip, models.ConflictActionReplace, models.ConflictActionMerge:
	default:
		writeJSONError(w, http.StatusBadRequest, services.ErrorUnsupportedAction)
		return
	}

	fd, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
		return
	}

	resolved := 0
	invalidRows := invalidRowNumbers(fd.InvalidRows)
	for index, row := range fd.Rows {
		if _, invalid := invalidRows[fileRowNumber(fd, index)]; invalid {
			continue
		}

		phone := strings.TrimSpace(row[phoneColumn])
		if phone == "" {
			continue
		}

		existing, exists, err := h.contacts.GetContactByPhone(r.Context(), phone)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, services.ErrorConflictNotSolved)
			return
		}
		if !exists {
			continue
		}

		incoming := services.RowToContact(row, phone, fd.ID)
		incoming.SourceRow = fileRowNumber(fd, index)

		if services.ContactsEqual(existing, incoming) {
			continue
		}

		if err := h.contacts.ResolveConflict(r.Context(), phone, req.Action, incoming); err != nil {
			writeJSONError(w, http.StatusInternalServerError, services.ErrorConflictNotSolved)
			return
		}
		resolved++
	}

	writeJSON(w, http.StatusOK, ResolveAllResponse{
		Status:   "ok",
		Resolved: resolved,
		Action:   req.Action,
	})
}

func fileRowNumber(data models.FileData, index int) int {
	if index < len(data.RowNumbers) && data.RowNumbers[index] > 0 {
		return data.RowNumbers[index]
	}
	return index + 1
}

func invalidRowNumbers(rows []models.InvalidRow) map[int]struct{} {
	result := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		result[row.Row] = struct{}{}
	}
	return result
}
