package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
)

type sessionCtxKey struct{}

// CookieName имя cookie с session id
const CookieName = "session_id"

// Session гарантирует наличие сессии в context и cookie
func Session(uc session.Usecase, ttl time.Duration, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var existing *uuid.UUID
			if c, err := r.Cookie(CookieName); err == nil {
				if u, err := uuid.Parse(c.Value); err == nil {
					existing = &u
				}
			}

			s, err := uc.Ensure(r.Context(), existing)
			if err != nil {
				logger.ForCtx(r.Context()).Error("ensure session", "err", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"An unexpected error occurred"}}`))
				return
			}

			if existing == nil || *existing != s.ID {
				http.SetCookie(w, &http.Cookie{
					Name:     CookieName,
					Value:    s.ID.String(),
					Path:     "/",
					HttpOnly: true,
					Secure:   secure,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   int(ttl.Seconds()),
				})
			}

			ctx := context.WithValue(r.Context(), sessionCtxKey{}, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionFromCtx достаёт сессию из context
func SessionFromCtx(ctx context.Context) *usecasemodels.Session {
	s, _ := ctx.Value(sessionCtxKey{}).(*usecasemodels.Session)
	return s
}
