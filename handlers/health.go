package handlers

// health.go – проверка состояния сервера
// GET /api/health проверяет доступность хранилища и возвращает статус
// сервиса. Используется Docker для healthcheck и для быстрой диагностики
// при запуске.

import (
	"context"
	"net/http"
	"time"

	"task1/services"
	"task1/storage"
)

// HealthHandler – обработчик проверки здоровья сервиса.
// Содержит ссылку на хранилище для вызова Ping().
type HealthHandler struct {
	store storage.FileStore
}

// healthResponse – формат ответа для GET /api/health.
type healthResponse struct {
	Status  string `json:"status"`
	Storage string `json:"storage"` // memory или postgres
	Error   string `json:"error,omitempty"`
}

// RegisterHealthRoutes – регистрация маршрута на "/api/health"
func RegisterHealthRoutes(mux *http.ServeMux, store storage.FileStore) {
	handler := &HealthHandler{store: store}
	mux.HandleFunc("/api/health", handler.Health)
}

// Health – обработчик GET /api/health.
//
// Логика:
//  1. CORS: OPTIONS → 204 No Content.
//  2. Проверка метода: только GET, иначе 405.
//  3. Создание контекста с таймаутом 2 секунды, чтобы Ping не висел вечно.
//  4. Вызов Ping(ctx) у хранилища:
//     – nil → {"status":"ok","storage":"memory|postgres"} (200)
//     – ошибка → {"status":"degraded","storage":"...","error":"storage unavailable"} (503)
//  5. cancel() откладывается через defer – гарантированно очищает ресурсы
//     контекста при любом выходе из функции.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {

	// 1. CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// 2. Проверка метода
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, services.ErrorMethodNotAllowed)
		return
	}

	// 3. Контекст с таймаутом
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// 4. Формирование payload
	payload := healthResponse{
		Status:  "ok",
		Storage: h.store.Driver(),
	}

	// 5. Ping хранилища
	if err := h.store.Ping(ctx); err != nil {
		payload.Status = "degraded"
		payload.Error = "storage unavailable"
		writeJSON(w, http.StatusServiceUnavailable, payload)
		return
	}

	writeJSON(w, http.StatusOK, payload)
}
