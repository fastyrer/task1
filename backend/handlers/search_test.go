package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task1/backend/models"
	"task1/backend/storage"
)

func setupSearchTest(t *testing.T) (*SearchHandler, string) {
	t.Helper()

	store := storage.NewMemoryStorage()
	handler := &SearchHandler{store: store}

	data := models.FileData{
		Headers: []string{"Телефон", "Имя", "Город"},
		Rows: []map[string]string{
			{"Телефон": "+79990001122", "Имя": "Анна", "Город": "Москва"},
			{"Телефон": "+79990003344", "Имя": "Иван", "Город": "Казань"},
			{"Телефон": "+74951234567", "Имя": "Мария", "Город": "Москва"},
		},
	}

	fileID, err := store.SaveFileData(context.Background(), data)
	if err != nil {
		t.Fatalf("failed to save fixture data: %v", err)
	}

	return handler, fileID
}

func TestSearchSuccess(t *testing.T) {
	handler, fileID := setupSearchTest(t)

	req := searchRequest{
		FileID: fileID,
		Query:  "+7999",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/search", &buf)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload searchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.TotalMatches != 2 || payload.Returned != 2 {
		t.Fatalf("unexpected match counts: %#v", payload)
	}
	if payload.Rows[0].Row != 1 || payload.Rows[1].Row != 2 {
		t.Fatalf("unexpected rows: %#v", payload.Rows)
	}
	if payload.Rows[0].Matches[0].Column != "Телефон" {
		t.Fatalf("unexpected match column: %#v", payload.Rows[0].Matches)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	handler, fileID := setupSearchTest(t)

	req := searchRequest{
		FileID: fileID,
		Query:  "мОС",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/search", &buf)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload searchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.TotalMatches != 2 {
		t.Fatalf("expected 2 city matches, got %#v", payload)
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	handler, fileID := setupSearchTest(t)

	req := searchRequest{
		FileID: fileID,
		Query:  "   ",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/search", &buf)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSearchFileNotFound(t *testing.T) {
	handler, _ := setupSearchTest(t)

	req := searchRequest{
		FileID: "missing",
		Query:  "+7999",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/search", &buf)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSearchLimit(t *testing.T) {
	handler, fileID := setupSearchTest(t)

	req := searchRequest{
		FileID: fileID,
		Query:  "москва",
		Limit:  1,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/search", &buf)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload searchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.TotalMatches != 2 || payload.Returned != 1 || !payload.Truncated {
		t.Fatalf("expected truncated response, got %#v", payload)
	}
}
