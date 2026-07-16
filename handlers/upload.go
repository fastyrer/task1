// Package handlers содержит HTTP-обработчики для всех эндпоинтов приложения.
//
// upload.go – загрузка и парсинг файлов с данными клиентов.
// POST /api/upload принимает multipart/form-data с файлом (CSV, XLS, XLSX)
// и опциональным именем листа Excel. Парсит содержимое, определяет
// заголовки, типы колонок, нормализует телефоны/email/скидки,
// возвращает полный локальный черновик и не записывает его в PostgreSQL.

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"task1/models"
	"task1/services"
	"task1/utils"
)

// Ограничения по размеру
const (
	defaultMaxUploadSize = 20 << 20
)

// UploadHandler не содержит хранилище: проверка файла должна быть stateless.
type UploadHandler struct{}

// ErrorResponse – единый JSON-формат ошибки всех API-обработчиков.
type ErrorResponse struct {
	Error string `json:"error" example:"Некорректный запрос"`
}

// RegisterUploadRoutes – регистрация stateless-маршрута проверки файла.
func RegisterUploadRoutes(mux *http.ServeMux) {
	handler := &UploadHandler{}
	mux.HandleFunc("/api/upload", handler.Upload)
}

// Upload – основной метод, обрабатывает POST-запросы с файлами.
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

	// Обработка CORS (предварительный запрос), возвращает ошибку 204
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Проверка на метод POST, иначе ошибка 405
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	uploadLimit := maxUploadSize()
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
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
		defer r.MultipartForm.RemoveAll()
	}

	// Получение файла из формы
	/*
		file – интерфейс для чтения файла
		header – метаданные файла
		err – ошибка
	*/
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorFileAbsent)
		return
	}
	defer file.Close()

	// Проверка размера файла
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

	importID, err := utils.GenerateUUID()
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

// writeJSON устанавливает заголовок Content-Type, HTTP-статус, кодирует payload
// в JSON и отправляет
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeJSONError – оборачивает сообщение в структуру ErrorResponse.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
