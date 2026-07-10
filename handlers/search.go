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

// searchRequest – данные запроса 
type searchRequest struct {
	FileID string `json:"fileId"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
}

// searchMatch – найденная ячейка
type searchMatch struct {
	Column string `json:"column"`
	Value  string `json:"value"`
}

// searchRow – строка с результатом поиска
type searchRow struct {
	Row     int               `json:"row"`
	Values  map[string]string `json:"values"` // Все строка целиком
	Matches []searchMatch     `json:"matches"` // Массив совпадений (какие колонки совпали у данной строки)
}

// searchResponse – данные ответа
type searchResponse struct {
	Query        string      `json:"query"`
	Headers      []string    `json:"headers"`
	Rows         []searchRow `json:"rows"`	
	TotalMatches int         `json:"totalMatches"` // Сколько всего совпадений
	Returned     int         `json:"returned"`	// Сколько вернули строк
	Limit        int         `json:"limit"`	
	Truncated    bool        `json:"truncated"`	// Обрезали ли количество
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
	var req searchRequest
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
	data, ok, err := h.store.GetFileData(r.Context(), req.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, services.ErrorFileNotOpened)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, services.ErrorFileNotFound)
		return
	}

	// 6. Поиск и формирование ответа
	resp := searchFileData(data, query, searchLimit(req.Limit))
	writeJSON(w, http.StatusOK, resp)
}

// searchFileData – основная функция поиска
func searchFileData(data models.FileData, query string, limit int) searchResponse {
	normalizedQuery := strings.ToLower(query)
	rows := make([]searchRow, 0)
	totalMatches := 0

	for index, row := range data.Rows {
		matches := rowMatches(data.Headers, row, normalizedQuery)
		if len(matches) == 0 {
			continue
		}

		totalMatches++
		if len(rows) >= limit {
			continue
		}

		rows = append(rows, searchRow{
			Row:     index + 1,
			Values:  row,
			Matches: matches,
		})
	}

	return searchResponse{
		Query:        query,
		Headers:      data.Headers,
		Rows:         rows,
		TotalMatches: totalMatches,
		Returned:     len(rows),
		Limit:        limit,
		Truncated:    totalMatches > len(rows),
	}
}

// rowMatches – проходит по всем колонкам строки и ищет совпадения
// вернет слайс с совпавшими колонками
func rowMatches(headers []string, row map[string]string, normalizedQuery string) []searchMatch {
	matches := make([]searchMatch, 0)
	for _, header := range headers {
		value := row[header]
		if strings.Contains(strings.ToLower(value), normalizedQuery) {
			matches = append(matches, searchMatch{
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
