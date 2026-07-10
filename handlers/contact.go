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
		writeJSONError(w, http.StatusBadRequest, services.ErrUnsupportedFormat.Error())
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

	phoneColumn := findPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
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

	phoneColumn := findPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorPhoneColNotFound)
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
