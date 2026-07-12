// main.go – точка входа в приложение.
//
// Создаёт хранилище (через STORAGE_DRIVER из окружения), регистрирует
// все HTTP-маршруты, оборачивает в CORS-мидлвару и запускает HTTP-сервер.
//
// Порядок инициализации:
//  1. context.Background() – корневой контекст для всего приложения
//  2. storage.NewFromEnv(ctx) – выбор драйвера (memory / postgres)
//  3. http.NewServeMux() – мультиплексор маршрутов
//  4. Register*Routes – регистрация всех обработчиков
//  5. registerFrontend(mux) – раздача статики (frontend/index.html)
//  6. http.ListenAndServe(addr, withCORS(mux)) – запуск сервера
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"task1/handlers"
	"task1/storage"
)

func main() {
	ctx := context.Background()

	// Создание хранилища: память или PostgreSQL (зависит от STORAGE_DRIVER)
	store, err := storage.NewFromEnv(ctx)
	if err != nil {
		log.Fatal(err) // выход, если не удалось подключиться к БД
	}
	defer store.Close() // закрытие пула соединений при завершении

	mux := http.NewServeMux()

	// Регистрация всех эндпоинтов
	handlers.RegisterHealthRoutes(mux, store)       // GET /api/health
	handlers.RegisterUploadRoutes(mux, store)       // POST /api/upload
	handlers.RegisterNotificationRoutes(mux, store) // POST /api/preview, /api/export
	handlers.RegisterSearchRoutes(mux, store)       // POST /api/search
	handlers.RegisterContactRoutes(mux, store, store) // POST /api/contacts/*, /api/rows/fix
	registerFrontend(mux)                            // GET / → frontend/index.html

	// Порт из окружения или 8080 по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("storage driver: %s", store.Driver())
	log.Printf("server started at http://localhost%s", addr)

	// Запуск HTTP-сервера с CORS-обёрткой
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

// registerFrontend подключает раздачу статических файлов фронтенда.
//
// Ищет папку frontend/ в текущей директории или на уровень выше
// (чтобы работало и из корня проекта, и из папки backend/).
// Если папка не найдена – все запросы на / возвращают 404.
func registerFrontend(mux *http.ServeMux) {
	frontendDir := findFrontendDir()
	if frontendDir == "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		return
	}

	fileServer := http.FileServer(http.Dir(frontendDir))
	mux.Handle("/", fileServer)
}

// findFrontendDir ищет существующую директорию с фронтендом.
//
// Сначала проверяет "frontend" в текущей папке (запуск из корня),
// затем "../frontend" (запуск из папки backend/).
func findFrontendDir() string {
	candidates := []string{
		"frontend",
		filepath.Join("..", "frontend"),
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// withCORS оборачивает http.Handler, добавляя CORS-заголовки.
//
// Разрешает:
//   - любые источники (Access-Control-Allow-Origin: *)
//   - методы GET, POST, OPTIONS
//   - заголовок Content-Type
//
// OPTIONS-запросы (preflight) сразу завершаются с 204 No Content.
// Остальные запросы передаются дальше по цепочке.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
