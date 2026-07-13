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
	"time"

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

	// Регистрация всех эндпоинтов
	handlers.RegisterHealthRoutes(mux, store)         // GET /api/health
	handlers.RegisterUploadRoutes(mux, store)         // POST /api/upload
	handlers.RegisterNotificationRoutes(mux, store)   // POST /api/preview, /api/export
	handlers.RegisterSearchRoutes(mux, store)         // POST /api/search
	handlers.RegisterContactRoutes(mux, store, store) // POST /api/contacts/*, /api/rows/fix
	registerFrontend(mux)                             // GET / → frontend/index.html

	// Порт из окружения или 8080 по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("storage driver: postgres")
	log.Printf("server started at http://localhost%s", addr)

	// Запуск HTTP-сервера с CORS-обёрткой
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
