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
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task1/handlers"
	"task1/storage"
)

//go:embed index.html
var frontendHTML []byte

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Режим migrate не запускает HTTP-сервер и поддерживает направления up/down.
	if len(os.Args) >= 2 && os.Args[1] == "migrate" {
		direction := "up"
		if len(os.Args) == 3 {
			direction = os.Args[2]
		} else if len(os.Args) > 3 {
			log.Fatal("usage: server migrate [up|down]")
		}

		switch direction {
		case "up":
			if err := storage.MigrateFromEnv(ctx); err != nil {
				log.Fatal(err)
			}
			log.Print("postgres migrations applied")
		case "down":
			if err := storage.RollbackMigrationFromEnv(ctx); err != nil {
				log.Fatal(err)
			}
			log.Print("postgres migration rolled back")
		default:
			log.Fatal("usage: server migrate [up|down]")
		}
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

	// Порт из окружения или 8080 по умолчанию.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Print("storage driver: postgres")
	log.Printf("server started at http://localhost%s", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
			_ = server.Close()
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped: %v", err)
		}
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
