// Command server запускает HTTP-приложение или выполняет одну миграцию схемы.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"task1/internal/handlers"
	"task1/internal/storage"
	"task1/internal/web"
)

// Шаблоны рассылки используют {{Поле}}, поэтому Swag получает другие разделители.
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo main.go --dir .,../../internal/handlers,../../internal/models --output ../../docs --parseInternal -td "[[,]]"

// @title Task1 Client Data API
// @version 1.0.0
// @description API для проверки клиентских CSV/XLS/XLSX, подтверждённого импорта контактов и подготовки рассылок.
// @description Черновик хранится в браузере; PostgreSQL изменяет финальный атомарный импорт и явное ручное сохранение контакта.
// @BasePath /
// @schemes http https
// @accept json
// @produce json
// @tag.name Health
// @tag.description Состояние приложения и подключение к PostgreSQL
// @tag.name Files
// @tag.description Stateless-загрузка и разбор файлов без записи в PostgreSQL
// @tag.name Imports
// @tag.description Проверка черновика, предпросмотр конфликтов и атомарный импорт
// @tag.name Contacts
// @tag.description Поиск, постраничный просмотр и ручное редактирование подтверждённого справочника PostgreSQL
// @tag.name Notifications
// @tag.description Предпросмотр и экспорт рассылки по всем контактам PostgreSQL
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run управляет жизненным циклом приложения; возврат ошибки гарантирует выполнение всех defer перед os.Exit.
func run() error {
	// Режим migrate не запускает HTTP-сервер и поддерживает направления up/down.
	if len(os.Args) >= 2 && os.Args[1] == "migrate" {
		ctx := context.Background()
		direction := "up"
		if len(os.Args) == 3 {
			direction = os.Args[2]
		} else if len(os.Args) > 3 {
			return fmt.Errorf("usage: server migrate [up|down]")
		}

		switch direction {
		case "up":
			if err := storage.MigrateFromEnv(ctx); err != nil {
				return err
			}
			log.Print("postgres migrations applied")
		case "down":
			if err := storage.RollbackMigrationFromEnv(ctx); err != nil {
				return err
			}
			log.Print("postgres migration rolled back")
		default:
			return fmt.Errorf("usage: server migrate [up|down]")
		}
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := validateRuntimeConfig(); err != nil {
		return err
	}

	// PostgreSQL обязателен; миграции выполняются отдельной командой.
	store, err := storage.NewFromEnv(ctx)
	if err != nil {
		return err
	}
	defer store.Close() // закрытие пула соединений при завершении

	mux := http.NewServeMux()

	// Регистрация всех эндпоинтов
	handlers.RegisterHealthRoutes(mux, store)       // GET /api/health
	handlers.RegisterUploadRoutes(mux)              // POST /api/upload, без записи в БД
	handlers.RegisterImportRoutes(mux, store)       // POST /api/imports/*
	handlers.RegisterContactRoutes(mux, store)      // GET /api/contacts, PUT /api/contacts/{uid}
	handlers.RegisterNotificationRoutes(mux, store) // POST /api/preview, /api/export
	registerSwagger(mux)                            // GET /swagger и /swagger/doc.json
	if err := registerFrontend(mux); err != nil {   // GET / и статические frontend-ресурсы
		return err
	}

	// Порт из окружения или 8080 по умолчанию.
	port, err := serverPort()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", port)
	log.Print("storage driver: postgres")
	log.Printf("server started at http://localhost:%d", port)

	server := &http.Server{
		Addr:              addr,
		Handler:           applicationHandler(mux),
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
			return err
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
	return nil
}

// serverPort возвращает проверенный TCP-порт и не допускает управляющие символы из окружения в адрес и логи.
func serverPort() (uint64, error) {
	value := strings.TrimSpace(os.Getenv("PORT"))
	if value == "" {
		value = "8080"
	}

	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return 0, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	return port, nil
}

// registerFrontend - подключает встроенные HTML, CSS и JavaScript к HTTP-серверу.
// Благодаря go:embed frontend остаётся внутри одного Go-бинарника.
func registerFrontend(mux *http.ServeMux) error {
	frontendRoot, err := web.FileSystem()
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
