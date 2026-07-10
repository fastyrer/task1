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
	store, err := storage.NewFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

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
