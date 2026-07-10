package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task1/storage"
)

func TestHealthOK(t *testing.T) {
	store := storage.NewMemoryStorage()
	mux := http.NewServeMux()
	RegisterHealthRoutes(mux, store)

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload healthResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Status != "ok" || payload.Storage != "memory" {
		t.Fatalf("unexpected health payload: %#v", payload)
	}
}
