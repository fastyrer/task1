package handlers

import (
	"encoding/json"
	"net/http"

	"task1/models"
	"task1/services"
	"task1/storage"
	"task1/utils"
)

type ContactHandler struct {
	store    storage.FileStore
	contacts storage.ContactStore
}

func RegisterContactRoutes(mux *http.ServeMux, store storage.FileStore, contacts storage.ContactStore) {
	h := &ContactHandler{store: store, contacts: contacts}
	mux.HandleFunc("/api/contacts/save", h.Save)
	mux.HandleFunc("/api/contacts/resolve", h.Resolve)
	mux.HandleFunc("/api/contacts/resolve-all", h.ResolveAll)
	mux.HandleFunc("/api/rows/fix", h.FixRows)
}

type fixRowsRequest struct {
	FileID string               `json:"fileId"`
	Rows   []services.FixRowInput `json:"rows"`
}

func (h *ContactHandler) FixRows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req fixRowsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	data, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось прочитать данные файла.")
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден.")
		return
	}

	phoneColumn := utils.DetectPhoneColumn(data.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, "Не найдена колонка с телефоном.")
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

type saveRequest struct {
	FileID string `json:"fileId"`
}

type saveResponse struct {
	Saved      int                   `json:"saved"`
	Skipped    int                   `json:"skipped"`
	Conflicts  []models.ConflictInfo `json:"conflicts,omitempty"`
}

func (h *ContactHandler) Save(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	data, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось прочитать данные файла.")
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден. Загрузите файл снова.")
		return
	}

	phoneColumn := utils.DetectPhoneColumn(data.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, "Не найдена колонка с телефоном.")
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

func (h *ContactHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req models.ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	if req.Phone == "" {
		writeJSONError(w, http.StatusBadRequest, "Не указан номер телефона.")
		return
	}

	switch req.Action {
	case models.ConflictActionSkip, models.ConflictActionReplace, models.ConflictActionMerge:
	default:
		writeJSONError(w, http.StatusBadRequest, "Неизвестное действие. Допустимые: skip, replace, merge.")
		return
	}

	fd, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось прочитать данные файла.")
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден.")
		return
	}

	phoneColumn := findPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, "Не найдена колонка с телефоном.")
		return
	}

	var incoming models.Contact
	var found bool
	for _, row := range fd.Rows {
		if row[phoneColumn] == req.Phone {
			incoming = services.RowToContact(row, req.Phone, fd.ID)
			found = true
			break
		}
	}

	if !found {
		writeJSONError(w, http.StatusNotFound, "Запись с таким телефоном не найдена в файле.")
		return
	}

	if err := h.contacts.ResolveConflict(r.Context(), req.Phone, req.Action, incoming); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось разрешить конфликт.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"phone":  req.Phone,
		"action": req.Action,
	})
}

func (h *ContactHandler) ResolveAll(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req models.BatchResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	switch req.Action {
	case models.ConflictActionSkip, models.ConflictActionReplace, models.ConflictActionMerge:
	default:
		writeJSONError(w, http.StatusBadRequest, "Неизвестное действие. Допустимые: skip, replace, merge.")
		return
	}

	fd, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось прочитать данные файла.")
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Файл не найден.")
		return
	}

	phoneColumn := findPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, "Не найдена колонка с телефоном.")
		return
	}

	resolved := 0
	for _, row := range fd.Rows {
		phone := row[phoneColumn]
		if phone == "" {
			continue
		}

		existing, exists, err := h.contacts.GetContactByPhone(r.Context(), phone)
		if err != nil {
			continue
		}
		if !exists {
			continue
		}

		incoming := services.RowToContact(row, phone, fd.ID)

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

func findPhoneColumn(headers []string) string {
	for _, h := range headers {
		if utils.ClassifyHeader(h) == utils.ColumnPhone {
			return h
		}
	}
	return ""
}
