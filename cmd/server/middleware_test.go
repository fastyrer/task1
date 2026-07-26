package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerPort(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      uint64
		wantError bool
	}{
		{name: "default", value: "", want: 8080},
		{name: "configured", value: "9090", want: 9090},
		{name: "zero", value: "0", wantError: true},
		{name: "too large", value: "65536", wantError: true},
		{name: "not a number", value: "8080-invalid", wantError: true},
		{name: "log injection", value: "8080\nforged", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PORT", test.value)
			port, err := serverPort()
			if test.wantError {
				if err == nil {
					t.Fatalf("serverPort() = %d, want error", port)
				}
				return
			}
			if err != nil {
				t.Fatalf("serverPort() error = %v", err)
			}
			if port != test.want {
				t.Fatalf("serverPort() = %d, want %d", port, test.want)
			}
		})
	}
}

func TestValidateRuntimeConfigRequiresProductionCredentials(t *testing.T) {
	t.Setenv("APP_ENV", productionEnvironment)
	t.Setenv("AUTH_USERNAME", "")
	t.Setenv("AUTH_PASSWORD", "")

	if err := validateRuntimeConfig(); err == nil {
		t.Fatal("production configuration without credentials must fail")
	}
}

func TestBasicAuthProtectsApplicationButNotHealth(t *testing.T) {
	t.Setenv("AUTH_USERNAME", "curator")
	t.Setenv("AUTH_PASSWORD", "strong-password")
	handler := withBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	authorizedRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	authorizedRequest.SetBasicAuth("curator", "strong-password")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d", authorized.Code)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusNoContent {
		t.Fatalf("health status = %d", health.Code)
	}
}

func TestCORSRejectsUntrustedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://trusted.example")
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://task1.local/api/preview", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("untrusted origin status = %d", response.Code)
	}
}

func TestCORSWildcardDoesNotBypassAuthentication(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	t.Setenv("AUTH_USERNAME", "curator")
	t.Setenv("AUTH_PASSWORD", "strong-password")
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://task1.local/api/preview", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("authenticated wildcard status = %d", response.Code)
	}
}

func TestSecurityHeadersAreAlwaysPresent(t *testing.T) {
	handler := withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	for _, header := range []string{
		"Content-Security-Policy",
		"Referrer-Policy",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Permissions-Policy",
	} {
		if response.Header().Get(header) == "" {
			t.Fatalf("security header %s is missing", header)
		}
	}
}
