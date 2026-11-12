package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticateRejectsInvalidCredentials(t *testing.T) {
	auth := NewAuth(nil, "test-secret-with-at-least-32-characters")

	tests := []struct {
		name   string
		header string
	}{
		{"missing token", ""},
		{"wrong scheme", "Basic abc"},
		{"malformed token", "Bearer invalid-token"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false

			handler := auth.Authenticate(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					called = true
				},
			))

			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			request.Header.Set("Authorization", tc.header)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", response.Code)
			}
			if called {
				t.Fatal("protected handler must not be called")
			}
		})
	}
}
