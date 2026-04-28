package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
)

// CSRFHeader имя заголовка с токеном
const CSRFHeader = "X-CSRF-Token"

// CSRF проверяет X-CSRF-Token для мутирующих методов
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		s := SessionFromCtx(r.Context())
		if s == nil {
			logger.ForCtx(r.Context()).Error("csrf: no session in ctx")
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
			return
		}

		got := r.Header.Get(CSRFHeader)
		if got == "" {
			writeJSONError(w, http.StatusForbidden, "csrf_missing", "CSRF token is required")
			return
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRF)) != 1 {
			writeJSONError(w, http.StatusForbidden, "csrf_invalid", "CSRF token is invalid")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
}
