package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "task1/docs" // Регистрирует сгенерированную Swagger-спецификацию.
)

// registerSwagger подключает интерактивную документацию раньше frontend-маршрута.
func registerSwagger(mux *http.ServeMux) {
	if !swaggerEnabled() {
		log.Print("swagger UI disabled")
		return
	}

	// Короткий адрес перенаправляется на каноническую страницу Swagger UI.
	mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, "/swagger/index.html", http.StatusTemporaryRedirect)
	})
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}

// swaggerEnabled оставляет документацию включённой локально и позволяет отключить её в production.
func swaggerEnabled() bool {
	value := strings.TrimSpace(os.Getenv("SWAGGER_ENABLED"))
	if value == "" {
		return true
	}

	enabled, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("invalid SWAGGER_ENABLED=%q; swagger UI remains enabled", value)
		return true
	}
	return enabled
}
