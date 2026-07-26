package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"task1/internal/identity"
	"task1/internal/models"
	"task1/internal/services"
)

const defaultMaxUploadSize = 20 << 20

// UploadHandler не содержит хранилище: проверка файла должна быть stateless.
type UploadHandler struct{}

// ErrorResponse задаёт единый JSON-формат ошибок API.
type ErrorResponse struct {
	Error string `json:"error" example:"Некорректный запрос"`
}

// RegisterUploadRoutes регистрирует stateless-маршрут проверки файла.
func RegisterUploadRoutes(mux *http.ServeMux) {
	handler := &UploadHandler{}
	mux.HandleFunc("/api/upload", handler.Upload)
}

// Upload проверяет размер, сигнатуру и содержимое загруженного файла.
// @Summary Проверить файл и создать локальный черновик
// @Description Принимает CSV, XLS или XLSX, нормализует все строки и возвращает их браузеру. PostgreSQL не изменяется.
// @Tags Files
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV/XLS/XLSX-файл с клиентскими данными"
// @Param sheet formData string false "Имя листа XLS/XLSX; если не задано, используется первый лист"
// @Success 200 {object} models.ImportValidationResult
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/upload [post]
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	uploadLimit := maxUploadSize()
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
	// Тело уже ограничено MaxBytesReader; gosec не распознаёт эту связь между вызовами.
	//nolint:gosec // G120: ParseMultipartForm не может прочитать больше uploadLimit.
	if err := r.ParseMultipartForm(uploadLimit); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, services.ErrorFileExcessiveSize+formatUploadSize(uploadLimit))
			return
		}
		writeJSONError(w, http.StatusBadRequest, services.ErrorFileNotOpened)
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorFileAbsent)
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size == 0 {
		writeJSONError(w, http.StatusBadRequest, services.ErrEmptyFile.Error())
		return
	}
	if header.Size > uploadLimit {
		writeJSONError(w, http.StatusRequestEntityTooLarge, services.ErrorFileExcessiveSize+formatUploadSize(uploadLimit))
		return
	}

	data, err := services.ParseByFilenameWithOptions(file, header.Filename, services.ParseOptions{
		SheetName: r.FormValue("sheet"),
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, userMessage(err))
		return
	}

	data.OriginalFilename = header.Filename
	data.Size = header.Size
	data.MIMEType = header.Header.Get("Content-Type")
	addMIMEWarning(&data)

	importID, err := identity.GenerateUUID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorImportIDNotCreated)
		return
	}

	writeJSON(w, http.StatusOK, services.NewImportValidation(data, importID))
}

func maxUploadSize() int64 {
	if value := strings.TrimSpace(os.Getenv("MAX_UPLOAD_SIZE_BYTES")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}

	if value := strings.TrimSpace(os.Getenv("MAX_UPLOAD_SIZE_MB")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return parsed << 20
		}
	}

	return defaultMaxUploadSize
}

func addMIMEWarning(data *models.FileData) {
	mimeType := strings.ToLower(strings.TrimSpace(data.MIMEType))
	if mimeType == "" || mimeType == "application/octet-stream" {
		return
	}

	if isExpectedMIME(data.Format, mimeType) {
		return
	}
	data.Warnings = append(data.Warnings, models.ProcessingWarning{
		Message: fmt.Sprintf("MIME-тип %s не соответствует формату %s.", data.MIMEType, strings.ToUpper(data.Format)),
	})
	data.Stats.WarningCount = len(data.Warnings)
}

func isExpectedMIME(format string, mimeType string) bool {
	switch format {
	case "csv":
		return mimeType == "text/csv" || mimeType == "application/csv" || strings.HasPrefix(mimeType, "text/plain")
	case "xls":
		return mimeType == "application/vnd.ms-excel"
	case "xlsx":
		return mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || mimeType == "application/zip"
	default:
		return true
	}
}

func formatUploadSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d Б", size)
	}
	if size < 1<<20 {
		return fmt.Sprintf("%.1f КБ", float64(size)/1024)
	}

	return fmt.Sprintf("%.1f МБ", float64(size)/(1<<20))
}

// userMessage не отдаёт клиенту неожиданные внутренние ошибки парсера.
func userMessage(err error) string {
	switch {
	case errors.Is(err, services.ErrUnsupportedFormat),
		errors.Is(err, services.ErrEmptyFile),
		errors.Is(err, services.ErrNoHeaders),
		errors.Is(err, services.ErrNoDataRows),
		errors.Is(err, services.ErrInvalidCSV),
		errors.Is(err, services.ErrInvalidExcel),
		errors.Is(err, services.ErrReadFile),
		errors.Is(err, services.ErrFileTypeMismatch),
		errors.Is(err, services.ErrInvalidEncoding),
		errors.Is(err, services.ErrSheetNotFound):
		return err.Error()
	default:
		return services.ErrorFileNotOpened
	}
}

// writeJSON отправляет JSON с указанным HTTP-статусом.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeJSONError оборачивает сообщение в ErrorResponse.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
