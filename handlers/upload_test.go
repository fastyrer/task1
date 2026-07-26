package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task1/models"
)

func TestUploadRejectsFileWithoutRequiredColumns(t *testing.T) {
	request := newCSVUploadRequest(t, "Телефон,ФИО\n+79991234567,Иван Иванов\n")
	response := httptest.NewRecorder()

	(&UploadHandler{}).Upload(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(payload.Error, "Отсутствуют обязательные колонки: «Email», «Скидка»") {
		t.Fatalf("upload error = %q", payload.Error)
	}
}

func TestUploadReturnsEditableDraftWhenRequiredValueIsEmpty(t *testing.T) {
	request := newCSVUploadRequest(t, "Телефон,ФИО,Email,Скидка\n+79991234567,,,\n")
	response := httptest.NewRecorder()

	(&UploadHandler{}).Upload(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload models.ImportValidationResult
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Stats.InvalidRowCount != 1 || payload.TableError == "" {
		t.Fatalf("unexpected validation result: %+v", payload)
	}
	if got := payload.InvalidRows[0].Errors[0]; got.Column != "ФИО" {
		t.Fatalf("unexpected field error: %+v", got)
	}
}

func newCSVUploadRequest(t *testing.T, content string) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("file", "contacts.csv")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
