// Package handlers реализует методы для приема и обработки файлов

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"task1/backend/services"
	"task1/backend/storage"
)

// Ограничения по размеру
const (
	maxUploadSize = 20 << 20 // 20 мб
	previewLimit  = 10 // показывать первые 10 строк
)

// UploadHandler – структура обработчика
type UploadHandler struct {
	store *storage.MemoryStorage // Указатель на хранилище
}

// uploadResponse – JSON теги
type uploadResponse struct {
	FileID      string              `json:"fileId"`
	Headers     []string            `json:"headers"`
	PreviewRows []map[string]string `json:"previewRows"`
}

// errorResponse – возврат ошибок в JSON-формате
type errorResponse struct {
	Error string `json:"error"`
}

// RegisterUploadRoutes – регистрация маршрута 
func RegisterUploadRoutes(mux *http.ServeMux, store *storage.MemoryStorage) {
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

	// Ограничение размера файла через maxUploadSize
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// Парсинг multipart-формы
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
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

	// Парсинг файла
	/*
		data – структура с заголовками и данными
		err – ошибка парсинга
	*/
	data, err := services.ParseByFilename(file, header.Filename)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, userMessage(err))
		return
	}

	// Сохранение данных в памяти
	// fileID – содержит уникальный ID
	fileID := h.store.SaveFileData(data)
	data.ID = fileID

	// Возврат ответа
	/*
		FileID – идентификация файла в следующих запросах
		Headers – список всех заголовков
		PreviewRows – первые 10 строк (предпросмотр)
	*/
	writeJSON(w, http.StatusOK, uploadResponse{
		FileID:      fileID,
		Headers:     data.Headers,
		PreviewRows: previewRows(data.Rows),
	})
}

//previewRows – возвращает первые 10 строк, либо все строки, если их < 10
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
		errors.Is(err, services.ErrReadFile):
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
