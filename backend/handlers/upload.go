package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"task1/backend/services"
	"task1/backend/storage"
)

const (
	maxUploadSize = 20 << 20
	previewLimit  = 10
)

type UploadHandler struct {
	store *storage.MemoryStorage
}

type uploadResponse struct {
	FileID      string              `json:"fileId"`
	Headers     []string            `json:"headers"`
	PreviewRows []map[string]string `json:"previewRows"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func RegisterUploadRoutes(mux *http.ServeMux, store *storage.MemoryStorage) {
	handler := &UploadHandler{store: store}
	mux.HandleFunc("/api/upload", handler.Upload)
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Не удалось прочитать файл.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Файл не передан.")
		return
	}
	defer file.Close()

	if header.Size == 0 {
		writeJSONError(w, http.StatusBadRequest, services.ErrEmptyFile.Error())
		return
	}

	data, err := services.ParseByFilename(file, header.Filename)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, userMessage(err))
		return
	}

	fileID := h.store.SaveFileData(data)
	data.ID = fileID

	writeJSON(w, http.StatusOK, uploadResponse{
		FileID:      fileID,
		Headers:     data.Headers,
		PreviewRows: previewRows(data.Rows),
	})
}

func previewRows(rows []map[string]string) []map[string]string {
	limit := previewLimit
	if len(rows) < limit {
		limit = len(rows)
	}

	return rows[:limit]
}

func userMessage(err error) string {
	switch {
	case errors.Is(err, services.ErrUnsupportedFormat),
		errors.Is(err, services.ErrEmptyFile),
		errors.Is(err, services.ErrNoHeaders),
		errors.Is(err, services.ErrNoDataRows),
		errors.Is(err, services.ErrInvalidCSV),
		errors.Is(err, services.ErrInvalidExcel),
		errors.Is(err, services.ErrReadFile):
		return err.Error()
	default:
		return err.Error()
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
