package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New создаёт JSON-логгер
func New(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// Inject кладёт логгер в context
func Inject(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// ForCtx достаёт логгер из context
func ForCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
