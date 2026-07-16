// Package handlers содержит HTTP-обработчики безопасного импорта.
//
// import.go разделяет read-only проверку и единственную записывающую операцию:
// POST /api/imports/validate повторно проверяет локальный черновик,
// POST /api/imports/preview только сравнивает контакты с PostgreSQL,
// POST /api/imports/commit атомарно сохраняет подтверждённый импорт.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"task1/models"
	"task1/services"
	"task1/storage"
)

// ImportHandler получает единый контракт read/write-операций завершённого импорта.
type ImportHandler struct {
	store storage.ImportStore
}

type ValidateImportRequest struct {
	Draft models.ImportDraft `json:"draft"`
}

type PreviewImportRequest struct {
	Draft models.ImportDraft `json:"draft"`
}

type CommitImportRequest struct {
	Draft     models.ImportDraft      `json:"draft"`
	Decisions []models.ImportDecision `json:"decisions"`
}

// RegisterImportRoutes регистрирует новый жизненный цикл локального черновика.
func RegisterImportRoutes(mux *http.ServeMux, store storage.ImportStore) {
	handler := &ImportHandler{store: store}
	mux.HandleFunc("/api/imports/validate", handler.Validate)
	mux.HandleFunc("/api/imports/preview", handler.Preview)
	mux.HandleFunc("/api/imports/commit", handler.Commit)
}

// Validate повторно нормализует отредактированный черновик без обращения к БД.
// @Summary Повторно проверить локальный черновик
// @Description Нормализует строки после редактирования и возвращает новую диагностику. PostgreSQL не изменяется.
// @Tags Imports
// @Accept json
// @Produce json
// @Param request body ValidateImportRequest true "Локальный черновик"
// @Success 200 {object} models.ImportValidationResult
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/imports/validate [post]
func (h *ImportHandler) Validate(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}

	var request ValidateImportRequest
	if !decodeImportJSON(w, r, &request) {
		return
	}
	data, err := services.ValidateImportDraft(request.Draft)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, services.ValidationResult(data))
}

// Preview сравнивает валидные строки с актуальными контактами и не выполняет INSERT/UPDATE.
// @Summary Предпросмотр последствий импорта
// @Description Возвращает новые, совпадающие и конфликтующие контакты. Операция выполняет только чтение PostgreSQL.
// @Tags Imports
// @Accept json
// @Produce json
// @Param request body PreviewImportRequest true "Проверенный локальный черновик"
// @Success 200 {object} models.ImportPreviewResult
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/imports/preview [post]
func (h *ImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}

	var request PreviewImportRequest
	if !decodeImportJSON(w, r, &request) {
		return
	}
	data, err := services.ValidateImportDraft(request.Draft)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := services.PreviewImport(r.Context(), h.store, data)
	if errors.Is(err, services.ErrDraftHasInvalidRows) || errors.Is(err, services.ErrPhoneColumnNotFound) {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

// Commit повторяет проверку и передаёт PostgreSQL полностью подготовленную транзакцию.
// @Summary Импортировать подтверждённые контакты
// @Description Повторно проверяет черновик и одной транзакцией сохраняет файл, строки, контакты и решения конфликтов.
// @Tags Imports
// @Accept json
// @Produce json
// @Param request body CommitImportRequest true "Черновик и решения всех конфликтов"
// @Success 200 {object} models.ImportCommitResult
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/imports/commit [post]
func (h *ImportHandler) Commit(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}

	var request CommitImportRequest
	if !decodeImportJSON(w, r, &request) {
		return
	}
	data, err := services.ValidateImportDraft(request.Draft)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := services.PreviewImport(r.Context(), h.store, data)
	if errors.Is(err, services.ErrDraftHasInvalidRows) || errors.Is(err, services.ErrPhoneColumnNotFound) {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorContactsNotRead)
		return
	}
	contacts, decisions, err := services.PrepareImport(data, preview, request.Decisions)
	if err != nil {
		if errors.Is(err, services.ErrPreviewOutdated) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.store.CommitImport(r.Context(), data, contacts, decisions)
	switch {
	case errors.Is(err, storage.ErrImportAlreadyCommitted):
		writeJSONError(w, http.StatusConflict, err.Error())
	case errors.Is(err, storage.ErrImportChanged):
		writeJSONError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotSaved)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return false
	}
	return true
}

// decodeImportJSON ограничивает JSON четырьмя максимальными размерами исходного файла.
// Запас нужен потому, что JSON-представление таблицы больше CSV/XLSX.
func decodeImportJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize()*4)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return false
	}
	return true
}
