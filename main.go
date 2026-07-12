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

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

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

	// OpenAPI config with Huma v2
	config := huma.DefaultConfig("Task1 API", "1.0.0")
	config.Info.Description = "Веб-приложение для загрузки CSV/XLS/XLSX, чтения заголовков, preview, валидации данных и генерации уведомлений по шаблону."
	config.Servers = []*huma.Server{{URL: "http://localhost:8080"}}
	config.OpenAPIPath = "/openapi"
	config.DocsPath = "/docs"
	config.SchemasPath = "/schemas"

	api := humago.New(mux, config)

	// Huma-style handlers (OpenAPI docs auto-generated)
	handlers.RegisterHumaRoutes(api, store, store)

	// Legacy handlers are still present in handlers/ package for tests.
	// If you need them, uncomment below:
	// handlers.RegisterHealthRoutes(mux, store)
	// handlers.RegisterUploadRoutes(mux, store)
	// handlers.RegisterNotificationRoutes(mux, store)
	// handlers.RegisterSearchRoutes(mux, store)
	// handlers.RegisterContactRoutes(mux, store, store)

	registerFrontend(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("storage driver: %s", store.Driver())
	log.Printf("server started at http://localhost%s", addr)
	log.Printf("OpenAPI docs at http://localhost%s/docs", addr)

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
