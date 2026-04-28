package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
)

// Recovery перехватывает panic и отдаёт 500
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.ForCtx(r.Context()).Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"An unexpected error occurred"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
