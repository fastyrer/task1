package handlers

import (
	"context"
	"net/http"
	"time"

	"task1/internal/services"
	"task1/internal/storage"
)

// HealthHandler проверяет доступность обязательного хранилища.
type HealthHandler struct {
	store storage.HealthStore
}

// HealthResponse описывает состояние приложения и PostgreSQL.
type HealthResponse struct {
	Status  string `json:"status" enums:"ok,degraded" example:"ok"`
	Storage string `json:"storage" enums:"postgres" example:"postgres"`
	Error   string `json:"error,omitempty" example:"storage unavailable"`
}

// RegisterHealthRoutes регистрирует endpoint проверки готовности.
func RegisterHealthRoutes(mux *http.ServeMux, store storage.HealthStore) {
	handler := &HealthHandler{store: store}
	mux.HandleFunc("/api/health", handler.Health)
}

// Health возвращает degraded, если PostgreSQL не отвечает за две секунды.
//
// @Summary Проверить состояние приложения
// @Description Проверяет доступность PostgreSQL с таймаутом две секунды.
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} HealthResponse
// @Router /api/health [get]
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

	payload := HealthResponse{
		Status:  "ok",
		Storage: "postgres",
	}

	if err := h.store.Ping(ctx); err != nil {
		payload.Status = "degraded"
		payload.Error = "storage unavailable"
		writeJSON(w, http.StatusServiceUnavailable, payload)
		return
	}

	writeJSON(w, http.StatusOK, payload)
}
