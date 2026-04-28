package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
)

// Logger пишет access-лог и обогащает логгер в context
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			l := base.With("request_id", RequestIDFromCtx(r.Context()))
			ctx := logger.Inject(r.Context(), l)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r.WithContext(ctx))

			l.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if sw.wroteHeader {
		return
	}
	sw.status = code
	sw.wroteHeader = true
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.WriteHeader(http.StatusOK)
	}
	return sw.ResponseWriter.Write(b)
}
