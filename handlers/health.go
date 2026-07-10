package handlers

import (
	"context"
	"net/http"
	"time"

	"task1/services"
	"task1/storage"
)

type HealthHandler struct {
	store storage.FileStore
}

type healthResponse struct {
	Status  string `json:"status"`
	Storage string `json:"storage"`
	Error   string `json:"error,omitempty"`
}

func RegisterHealthRoutes(mux *http.ServeMux, store storage.FileStore) {
	handler := &HealthHandler{store: store}
	mux.HandleFunc("/api/health", handler.Health)
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	payload := healthResponse{
		Status:  "ok",
		Storage: h.store.Driver(),
	}
	if err := h.store.Ping(ctx); err != nil {
		payload.Status = "degraded"
		payload.Error = "storage unavailable"
		writeJSON(w, http.StatusServiceUnavailable, payload)
		return
	}

	writeJSON(w, http.StatusOK, payload)
}
