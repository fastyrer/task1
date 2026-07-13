// main.go – точка входа в приложение.
//
// Подключается к PostgreSQL через DATABASE_URL, регистрирует
// все HTTP-маршруты, оборачивает в CORS-мидлвару и запускает HTTP-сервер.
//
// Порядок инициализации:
//  1. context.Background() – корневой контекст для всего приложения
//  2. storage.NewFromEnv(ctx) – подключение к уже мигрированной PostgreSQL
//  3. http.NewServeMux() – мультиплексор маршрутов
//  4. Register*Routes – регистрация всех обработчиков
//  5. registerFrontend(mux) – раздача встроенного index.html
//  6. http.ListenAndServe(addr, withCORS(mux)) – запуск сервера
package main

import (
	"bytes"
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"task1/handlers"
	"task1/storage"
)

//go:embed index.html
var frontendHTML []byte

func main() {
	ctx := context.Background()
	// Режим migrate не запускает HTTP-сервер: он применяет схему и сразу завершается.
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		if err := storage.MigrateFromEnv(ctx); err != nil {
			log.Fatal(err)
		}
		log.Print("postgres migrations applied")
		return
	}

	// PostgreSQL обязателен; миграции выполняются отдельной командой.
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
	log.Printf("storage driver: postgres")
	log.Printf("server started at http://localhost%s", addr)
	log.Printf("OpenAPI docs at http://localhost%s/docs", addr)

	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

// registerFrontend - отдаёт встроенный index.html независимо от рабочей директории.
// Благодаря go:embed фронтенд находится внутри одного Go-бинарника.
func registerFrontend(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(
			w,
			r,
			"index.html",
			time.Time{},
			bytes.NewReader(frontendHTML),
		)
	})
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
