package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"task1/backend/models"
	"task1/backend/storage"
)

const defaultSearchResultLimit = 1000

type SearchHandler struct {
	store storage.FileStore
}

type searchRequest struct {
	FileID string `json:"fileId"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
}

type searchMatch struct {
	Column string `json:"column"`
	Value  string `json:"value"`
}

type searchRow struct {
	Row     int               `json:"row"`
	Values  map[string]string `json:"values"`
	Matches []searchMatch     `json:"matches"`
}

type searchResponse struct {
	Query        string      `json:"query"`
	Headers      []string    `json:"headers"`
	Rows         []searchRow `json:"rows"`
	TotalMatches int         `json:"totalMatches"`
	Returned     int         `json:"returned"`
	Limit        int         `json:"limit"`
	Truncated    bool        `json:"truncated"`
}

func RegisterSearchRoutes(mux *http.ServeMux, store storage.FileStore) {
	handler := &SearchHandler{store: store}
	mux.HandleFunc("/api/search", handler.Search)
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Метод не поддерживается.")
		return
	}

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неверный формат запроса.")
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSONError(w, http.StatusBadRequest, "Введите строку поиска.")
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

	resp := searchFileData(data, query, searchLimit(req.Limit))
	writeJSON(w, http.StatusOK, resp)
}

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

func searchLimit(requested int) int {
	maxLimit := maxSearchResultLimit()
	if requested <= 0 || requested > maxLimit {
		return maxLimit
	}

	return requested
}

func maxSearchResultLimit() int {
	value := strings.TrimSpace(os.Getenv("SEARCH_RESULT_LIMIT"))
	if value == "" {
		return defaultSearchResultLimit
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultSearchResultLimit
	}

	return parsed
}
