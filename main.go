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
//  5. registerFrontend(mux) – раздача встроенной директории frontend
//  6. http.ListenAndServe(addr, withCORS(mux)) – запуск сервера
package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"task1/handlers"
	"task1/storage"
)

// Шаблоны рассылки используют {{Поле}}, поэтому Swag получает другие разделители.
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo main.go --output docs --parseInternal -td "[[,]]"

//go:embed frontend
var frontendFiles embed.FS

// @title Task1 Client Data API
// @version 1.0.0
// @description API для загрузки клиентских CSV/XLS/XLSX, проверки и поиска строк, ведения контактов и подготовки рассылок.
// @description Все операции с файлами и контактами используют PostgreSQL как единственное хранилище.
// @BasePath /
// @schemes http https
// @accept json
// @produce json
// @tag.name Health
// @tag.description Состояние приложения и подключение к PostgreSQL
// @tag.name Files
// @tag.description Загрузка, разбор и исправление строк файлов
// @tag.name Search
// @tag.description Поиск по строкам ранее загруженного файла
// @tag.name Contacts
// @tag.description Сохранение контактов и разрешение конфликтов по телефону
// @tag.name Notifications
// @tag.description Предпросмотр и экспорт рассылки по всем контактам PostgreSQL
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
	registerSwagger(mux)                              // GET /swagger и /swagger/doc.json
	if err := registerFrontend(mux); err != nil {     // GET / и статические frontend-ресурсы
		log.Fatal(err)
	}

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
		// передача данных между каналами
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

// registerFrontend - подключает встроенные HTML, CSS и JavaScript к HTTP-серверу.
// Благодаря go:embed frontend остаётся внутри одного Go-бинарника.
func registerFrontend(mux *http.ServeMux) error {
	frontendRoot, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		return fmt.Errorf("open embedded frontend: %w", err)
	}

	mux.Handle("/", frontendHandler(frontendRoot))
	return nil
}

// frontendHandler отдаёт только существующие файлы и не показывает каталоги.
func frontendHandler(frontendRoot fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}

		content, err := fs.ReadFile(frontendRoot, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(
			w,
			r,
			name,
			time.Time{},
			bytes.NewReader(content),
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
