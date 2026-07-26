package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const productionEnvironment = "production"

// applicationHandler задаёт единый порядок HTTP-защиты:
// security headers -> CORS/CSRF -> authentication -> application routes.
func applicationHandler(next http.Handler) http.Handler {
	return withSecurityHeaders(withCORS(withBasicAuth(next)))
}

// validateRuntimeConfig не позволяет запустить production без аутентификации
// или с открытым для всех CORS. Миграционный режим эту проверку не выполняет.
func validateRuntimeConfig() error {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	username := strings.TrimSpace(os.Getenv("AUTH_USERNAME"))
	password := os.Getenv("AUTH_PASSWORD")

	if (username == "") != (password == "") {
		return fmt.Errorf("AUTH_USERNAME and AUTH_PASSWORD must be configured together")
	}
	if environment == productionEnvironment {
		if username == "" {
			return fmt.Errorf("AUTH_USERNAME and AUTH_PASSWORD are required in production")
		}
		for _, origin := range allowedCORSOrigins() {
			if origin == "*" || origin == "null" {
				return fmt.Errorf("wildcard and null CORS origins are forbidden in production")
			}
		}
	}
	return nil
}

// withBasicAuth включает HTTP Basic Auth, когда заданы обе переменные окружения.
// /api/health остаётся доступным оркестратору без учётных данных.
func withBasicAuth(next http.Handler) http.Handler {
	username := strings.TrimSpace(os.Getenv("AUTH_USERNAME"))
	password := os.Getenv("AUTH_PASSWORD")
	if username == "" || password == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		providedUsername, providedPassword, ok := r.BasicAuth()
		if !ok || !secureEqual(providedUsername, username) || !secureEqual(providedPassword, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="task1", charset="UTF-8"`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func secureEqual(provided, expected string) bool {
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}

// withCORS принимает same-origin запросы и явно перечисленные дополнительные
// origins. Запрос с недоверенным Origin отклоняется до бизнес-обработчика, что
// также защищает Basic Auth-сессию от cross-site form submission.
func withCORS(next http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{})
	for _, origin := range allowedCORSOrigins() {
		allowedOrigins[origin] = struct{}{}
	}
	authEnabled := strings.TrimSpace(os.Getenv("AUTH_USERNAME")) != ""

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			_, explicitlyAllowed := allowedOrigins[origin]
			_, wildcardConfigured := allowedOrigins["*"]
			wildcardAllowed := wildcardConfigured && !authEnabled
			if !sameRequestOrigin(r, origin) && !explicitlyAllowed && !wildcardAllowed {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			allowedOrigin := origin
			if wildcardAllowed {
				allowedOrigin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Add("Vary", "Origin")
			if allowedOrigin != "*" {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigins() []string {
	values := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func sameRequestOrigin(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Host, r.Host) {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(parsed.Scheme, scheme)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
