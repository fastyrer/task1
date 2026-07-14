// Package handlers содержит HTTP-обработчики для всех эндпоинтов.

// search.go – полнотекстовый поиск по загруженным данным.
// GET/POST /api/search принимает fileId, строку запроса и опциональный лимит,
// возвращает строки, где хотя бы одна колонка содержит подстроку запроса
// (регистронезависимо), с информацией о конкретных колонках-совпадениях.

package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"task1/models"
	"task1/services"
	"task1/storage"
)

// SearchHandler – внедрение зависимости через интерфейс FileStore
type SearchHandler struct {
	store storage.FileStore
}

// SearchRequest – данные запроса поиска по одному загруженному файлу.
type SearchRequest struct {
	FileID string `json:"fileId" validate:"required" example:"2f656bc0-6227-49d3-9d09-b2d59bd21c52"`
	Query  string `json:"query" validate:"required" example:"+79"`
	Limit  int    `json:"limit,omitempty" minimum:"1" example:"100"`
}

// SearchMatch – найденная ячейка строки.
type SearchMatch struct {
	Column string `json:"column" example:"Телефон"`
	Value  string `json:"value" example:"+79991234567"`
}

// SearchRow – полная строка файла с указанием совпавших ячеек.
type SearchRow struct {
	Row     int               `json:"row"`
	Values  map[string]string `json:"values"`  // Все строка целиком
	Matches []SearchMatch     `json:"matches"` // Массив совпадений (какие колонки совпали у данной строки)
}

// SearchResponse – результат поиска с информацией об ограничении выдачи.
type SearchResponse struct {
	Query        string      `json:"query" example:"+79"`
	Headers      []string    `json:"headers"`
	Rows         []SearchRow `json:"rows"`
	TotalMatches int         `json:"totalMatches" example:"17"` // Сколько всего совпадений
	Returned     int         `json:"returned" example:"17"`     // Сколько вернули строк
	Limit        int         `json:"limit" example:"100"`
	Truncated    bool        `json:"truncated" example:"false"` // Обрезали ли количество
}

// RegisterSearchRoutes – регистрация обработчиков для поисковых запросов
func RegisterSearchRoutes(mux *http.ServeMux, store storage.FileStore) {
	handler := &SearchHandler{store: store}
	mux.HandleFunc("/api/search", handler.Search) // Регистрация маршрута
}

// Search:
// 1. Обработка CORS (preflight)
// 2. Проверка HTTP-метода
// 3. Парсинг JSON-запроса
// 4. Валидация данных
// 5. Получение данных из хранилища
// 6. Поиск и формирование ответа
// @Summary Найти строки в файле
// @Description Ищет подстроку без учёта регистра во всех ячейках ранее загруженного файла. Возвращает полные строки и колонки совпадений для подсветки.
// @Tags Search
// @Accept json
// @Produce json
// @Param request body SearchRequest true "Параметры поиска"
// @Success 200 {object} SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/search [post]
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {

	// 1. Проверка CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// 2. Проверка метода на POST (только он поддерживается)
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	// 3. Парсинг JSON-запроса
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return
	}

	// 4. Валидация данных запроса
	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSONError(w, http.StatusBadRequest, services.ErrorEmptyRequestLine)
		return
	}

	// 5. Получение данных из хранилища
	limit := searchLimit(req.Limit)
	result, ok, err := h.store.SearchFileRows(r.Context(), req.FileID, query, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	// 6. Поиск и формирование ответа
	resp := searchStoredRows(result, query, limit)
	writeJSON(w, http.StatusOK, resp)
}

// searchStoredRows - преобразует строки, найденные PostgreSQL, в формат HTTP-ответа.
// PostgreSQL выбирает подходящие строки, а rowMatches отмечает конкретные ячейки для подсветки.
func searchStoredRows(result models.FileSearchResult, query string, limit int) SearchResponse {
	normalizedQuery := strings.ToLower(query)
	rows := make([]SearchRow, 0, len(result.Rows))
	for _, storedRow := range result.Rows {
		rows = append(rows, SearchRow{
			Row:     storedRow.Row,
			Values:  storedRow.Values,
			Matches: rowMatches(result.Headers, storedRow.Values, normalizedQuery),
		})
	}

	return SearchResponse{
		Query:        query,
		Headers:      result.Headers,
		Rows:         rows,
		TotalMatches: result.Total,
		Returned:     len(rows),
		Limit:        limit,
		Truncated:    result.Total > len(rows),
	}
}

// rowMatches – проходит по всем колонкам строки и ищет совпадения
// вернет слайс с совпавшими колонками
func rowMatches(headers []string, row map[string]string, normalizedQuery string) []SearchMatch {
	matches := make([]SearchMatch, 0)
	for _, header := range headers {
		value := row[header]
		if strings.Contains(strings.ToLower(value), normalizedQuery) {
			matches = append(matches, SearchMatch{
				Column: header,
				Value:  value,
			})
		}
	}

	return matches
}

// searchLimit – валидация лимита
func searchLimit(requested int) int {
	maxLimit := maxSearchResultLimit()
	if requested <= 0 || requested > maxLimit {
		return maxLimit
	}

	return requested
}

// maxSearchResultLimit – чтение лимита из .env
func maxSearchResultLimit() int {
	value := strings.TrimSpace(os.Getenv("SEARCH_RESULT_LIMIT"))
	if value == "" {
		return services.DefaultSearchResultLimit
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return services.DefaultSearchResultLimit
	}

	return parsed
}
