package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task1/models"
)

type importStoreStub struct {
	commitCalls int
}

func (stub *importStoreStub) FindContactsByPhones(_ context.Context, _ []string) (map[string]models.Contact, error) {
	return map[string]models.Contact{}, nil
}

func (stub *importStoreStub) CommitImport(
	_ context.Context,
	_ models.FileData,
	_ []models.Contact,
	_ map[string]models.ImportDecision,
) (models.ImportCommitResult, error) {
	stub.commitCalls++
	return models.ImportCommitResult{}, nil
}

func TestPreviewDoesNotCommitImport(t *testing.T) {
	store := &importStoreStub{}
	mux := http.NewServeMux()
	RegisterImportRoutes(mux, store)

	requestBody, err := json.Marshal(PreviewImportRequest{Draft: models.ImportDraft{
		ImportID:         "550e8400-e29b-41d4-a716-446655440000",
		OriginalFilename: "contacts.csv",
		Format:           "csv",
		HeaderRow:        1,
		Headers:          []string{"Телефон", "ФИО"},
		Rows: []models.ImportRow{{
			RowNumber: 2,
			Values:    map[string]string{"Телефон": "+79991234567", "ФИО": "Иван Иванов"},
		}},
	}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/imports/preview", bytes.NewReader(requestBody))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.commitCalls != 0 {
		t.Fatalf("preview called CommitImport %d time(s)", store.commitCalls)
	}
}
