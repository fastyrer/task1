// Package handlers содержит HTTP-обработчики для всех эндпоинтов приложения.
//
// contact.go – управление контактами: сохранение строк из файла как контактов,
// разрешение конфликтов (когда контакт с таким телефоном уже существует),
// и исправление невалидных строк через редактирование на фронте.

package handlers

import (
	"encoding/json"
	"net/http"

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

// fixRowsRequest – запрос на исправление невалидных строк
type fixRowsRequest struct {
	FileID string                 `json:"fileId"`
	Rows   []services.FixRowInput `json:"rows"`
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
func (h *ContactHandler) FixRows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req fixRowsRequest
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
		if err := services.FixAndSaveRow(r.Context(), h.contacts, row, data.Headers, phoneColumn, data.ID); err != nil {
			result.Failed = append(result.Failed, *err)
		} else {
			result.Fixed++
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// saveRequest – запрос на сохранение данных из файла
type saveRequest struct {
	FileID string `json:"fileId"`
}

// saveResponse – ответ на сохранение
type saveResponse struct {
	Saved     int                   `json:"saved"`               // Сколько сохранено
	Skipped   int                   `json:"skipped"`             // Сколько пропущено
	Conflicts []models.ConflictInfo `json:"conflicts,omitempty"` // конфликты
	// с существующими контактами
}

// Save – POST /api/contacts/save
//
// Проходит по всем валидным строкам файла и сохраняет их как контакты.
// Если контакт с таким телефоном уже существует и данные различаются –
// не сохраняет, а возвращает ConflictInfo с описанием расхождений.
//
// Фронт получает список конфликтов и предлагает пользователю выбрать действие:
// skip (пропустить), replace (заменить) или merge (слить).
func (h *ContactHandler) Save(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	var req saveRequest
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

	writeJSON(w, http.StatusOK, saveResponse{
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
		if row[phoneColumn] == req.Phone {
			incoming = services.RowToContact(row, req.Phone, fd.ID)
			// В аудит передаётся номер строки из файла, а не её позиция в slice.
			incoming.SourceRow = index + 1
			if index < len(fd.RowNumbers) && fd.RowNumbers[index] > 0 {
				incoming.SourceRow = fd.RowNumbers[index]
			}
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"phone":  req.Phone,
		"action": req.Action,
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
	for index, row := range fd.Rows {
		phone := row[phoneColumn]
		if phone == "" {
			continue
		}

		existing, exists, err := h.contacts.GetContactByPhone(r.Context(), phone)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
			return
		}
		if !exists {
			continue
		}

		incoming := services.RowToContact(row, phone, fd.ID)
		// Запоминаем исходную строку для contact_sources при массовом разрешении.
		incoming.SourceRow = index + 1
		if index < len(fd.RowNumbers) && fd.RowNumbers[index] > 0 {
			incoming.SourceRow = fd.RowNumbers[index]
		}

		if services.ContactsEqual(existing, incoming) {
			continue
		}

		if err := h.contacts.ResolveConflict(r.Context(), phone, req.Action, incoming); err != nil {
			continue
		}
		resolved++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"resolved": resolved,
		"action":   req.Action,
	})
}
