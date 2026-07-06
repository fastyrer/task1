package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"task1/backend/handlers"
	"task1/backend/storage"
)

func main() {
	mux := http.NewServeMux()

	handlers.RegisterUploadRoutes(mux, storage.DefaultStorage())
	registerFrontend(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("server started at http://localhost%s", addr)
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
