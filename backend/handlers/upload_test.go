package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"task1/backend/storage"
)

func TestUploadCSV(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "clients.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte("Телефон,Имя\n+79990001122,Анна\n"))
	_ = writer.Close()

	store := storage.NewMemoryStorage()
	mux := http.NewServeMux()
	RegisterUploadRoutes(mux, store)

	request := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload uploadResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.FileID == "" {
		t.Fatal("expected fileId in response")
	}
	if len(payload.Headers) != 2 || payload.Headers[0] != "Телефон" {
		t.Fatalf("unexpected headers: %#v", payload.Headers)
	}
	if payload.PreviewRows[0]["Имя"] != "Анна" {
		t.Fatalf("unexpected preview rows: %#v", payload.PreviewRows)
	}
	if payload.Format != "csv" {
		t.Fatalf("expected csv format, got %q", payload.Format)
	}
	if payload.DetectedMIMEType != "text/csv" {
		t.Fatalf("expected detected MIME text/csv, got %q", payload.DetectedMIMEType)
	}
	if payload.Stats.RowCount != 1 || payload.Stats.ValidRowCount != 1 {
		t.Fatalf("unexpected stats: %#v", payload.Stats)
	}

	stored, ok := store.GetFileData(payload.FileID)
	if !ok {
		t.Fatal("expected parsed file to be stored")
	}
	if len(stored.Rows) != 1 {
		t.Fatalf("expected stored row, got %#v", stored.Rows)
	}
	if stored.OriginalFilename != "clients.csv" {
		t.Fatalf("expected original filename to be stored, got %q", stored.OriginalFilename)
	}
}

func TestUploadWithoutFile(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	store := storage.NewMemoryStorage()
	mux := http.NewServeMux()
	RegisterUploadRoutes(mux, store)

	request := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestUploadRejectsTooLargeFile(t *testing.T) {
	t.Setenv("MAX_UPLOAD_SIZE_BYTES", "32")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "clients.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte("Телефон,Имя\n+79990001122,Анна\n"))
	_ = writer.Close()

	store := storage.NewMemoryStorage()
	mux := http.NewServeMux()
	RegisterUploadRoutes(mux, store)

	request := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d: %s", response.Code, response.Body.String())
	}
}

func TestUploadRejectsMismatchedFileContent(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "clients.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte("PK\x03\x04fake xlsx"))
	_ = writer.Close()

	store := storage.NewMemoryStorage()
	mux := http.NewServeMux()
	RegisterUploadRoutes(mux, store)

	request := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
}
