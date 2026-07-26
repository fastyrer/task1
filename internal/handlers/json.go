package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"task1/internal/services"
)

// decodeJSONBody применяет единые правила ко всем JSON-endpoint:
// ограничивает размер, запрещает неизвестные поля и принимает ровно один JSON-объект.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeJSONDecodeError(w, err)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
		return false
	}
	return true
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeJSONError(w, http.StatusRequestEntityTooLarge, services.ErrorRequestTooLarge)
		return
	}
	writeJSONError(w, http.StatusBadRequest, services.ErrorBadRequest)
}
