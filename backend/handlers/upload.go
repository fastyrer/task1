// Package handlers реализует методы для приема и обработки файлов

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"task1/backend/models"
	"task1/backend/services"
	"task1/backend/storage"
	"task1/backend/utils"
)

// Ограничения по размеру
const (
	defaultMaxUploadSize = 20 << 20
	previewLimit         = 10
)

// UploadHandler – структура обработчика
type UploadHandler struct {
	store storage.FileStore
}

// uploadResponse – JSON теги
type uploadResponse struct {
	FileID              string                     `json:"fileId"`
	OriginalFilename    string                     `json:"originalFilename,omitempty"`
	Size                int64                      `json:"size,omitempty"`
	MIMEType            string                     `json:"mimeType,omitempty"`
	DetectedMIMEType    string                     `json:"detectedMimeType,omitempty"`
	Format              string                     `json:"format,omitempty"`
	Encoding            string                     `json:"encoding,omitempty"`
	SheetName           string                     `json:"sheetName,omitempty"`
	Sheets              []string                   `json:"sheets,omitempty"`
	HeaderRow           int                        `json:"headerRow,omitempty"`
	Headers             []string                   `json:"headers"`
	PreviewRows         []map[string]string        `json:"previewRows"`
	Stats               models.ProcessingStats     `json:"stats"`
	Warnings            []models.ProcessingWarning `json:"warnings,omitempty"`
	InvalidRows         []models.InvalidRow        `json:"invalidRows,omitempty"`
	DetectedPhoneColumn string                     `json:"detectedPhoneColumn,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// RegisterUploadRoutes – регистрация маршрута
func RegisterUploadRoutes(mux *http.ServeMux, store storage.FileStore) {
	handler := &UploadHandler{store: store}
	mux.HandleFunc("/api/upload", handler.Upload)
}

// Upload – основной метод, обрабатывает POST-запросы с файлами
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {

	// Обработка CORS (предварительный запрос), возвращает ошибку 204
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Проверка на метод POST, иначе ошибка 405
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	uploadLimit := maxUploadSize()
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
	if err := r.ParseMultipartForm(uploadLimit); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Файл слишком большой. Максимальный размер: %s.", formatUploadSize(uploadLimit)))
			return
		}
		writeJSONError(w, http.StatusBadRequest, "Не удалось прочитать файл.")
		return
	}

	// Получение файла из формы
	/*
		file – интерфейс для чтения файла
		header – метаданные файла
		err – ошибка
	*/
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Файл не передан.")
		return
	}
	defer file.Close()

	// Проверка размера файла
	if header.Size == 0 {
		writeJSONError(w, http.StatusBadRequest, services.ErrEmptyFile.Error())
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

	fileID, err := h.store.SaveFileData(r.Context(), data)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Не удалось сохранить данные файла.")
		return
	}
	data.ID = fileID

	// Возврат ответа
	/*
		FileID – идентификация файла в следующих запросах
		Headers – список всех заголовков
		PreviewRows – первые 10 строк (предпросмотр)
	*/
	writeJSON(w, http.StatusOK, uploadResponse{
		FileID:              fileID,
		OriginalFilename:    data.OriginalFilename,
		Size:                data.Size,
		MIMEType:            data.MIMEType,
		DetectedMIMEType:    data.DetectedMIMEType,
		Format:              data.Format,
		Encoding:            data.Encoding,
		SheetName:           data.SheetName,
		Sheets:              data.Sheets,
		HeaderRow:           data.HeaderRow,
		Headers:             data.Headers,
		PreviewRows:         previewRows(data.Rows),
		Stats:               data.Stats,
		Warnings:            data.Warnings,
		InvalidRows:         data.InvalidRows,
		DetectedPhoneColumn: utils.DetectPhoneColumn(data.Headers),
	})
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

func previewRows(rows []map[string]string) []map[string]string {
	limit := previewLimit
	if len(rows) < limit {
		limit = len(rows)
	}

	return rows[:limit]
}

// userMessage выводит сообщение об ошибке для пользователя
// err – одна из возможных ошибок
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
		return err.Error()
	}
}

// writeJSON устанавливает заголовок Content-Type, HTTP-статус, кодирует payload
// в JSON и отправляет
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeJSONError – оборачивает сообщение в структуру errorResponse
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
