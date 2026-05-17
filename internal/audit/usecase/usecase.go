// Package usecase реализует чтение аудит-лога (списки и история по заказу).
// Запись (Recorder) идёт напрямую через repository, чтобы не строить лишнюю
// прослойку — usecase'ы фич сами зовут recorder.Record(...).
package usecase

import (
	"context"
	"log/slog"

	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	"github.com/google/uuid"
)

// LimitDefaults дефолты пагинации.
type LimitDefaults struct {
	// Default дефолтный размер страницы, если limit не задан.
	Default int
	// Max верхняя граница limit.
	Max int
}

// Reader сценарии чтения аудит-лога.
type Reader interface {
	List(ctx context.Context, f audit.Filter) ([]audit.Action, int, error)
	ListByOrder(ctx context.Context, orderID uuid.UUID, limit, offset int) ([]audit.Action, int, error)
}

type readerImpl struct {
	repo     audit.Reader
	defaults LimitDefaults
}

// New собирает Reader.
func New(repo audit.Reader, def LimitDefaults) Reader {
	if def.Default <= 0 {
		def.Default = 50
	}
	if def.Max <= 0 || def.Max < def.Default {
		def.Max = 200
	}
	return &readerImpl{repo: repo, defaults: def}
}

// List нормализует пагинацию и проксирует в repo.
func (u *readerImpl) List(ctx context.Context, f audit.Filter) ([]audit.Action, int, error) {
	f.Limit, f.Offset = u.normalizePagination(f.Limit, f.Offset)
	return u.repo.List(ctx, f)
}

// ListByOrder нормализует пагинацию и проксирует в repo.
func (u *readerImpl) ListByOrder(
	ctx context.Context,
	orderID uuid.UUID,
	limit, offset int,
) ([]audit.Action, int, error) {
	limit, offset = u.normalizePagination(limit, offset)
	return u.repo.ListByOrder(ctx, orderID, limit, offset)
}

// normalizePagination применяет дефолты и верхнюю границу.
func (u *readerImpl) normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = u.defaults.Default
	}
	if limit > u.defaults.Max {
		limit = u.defaults.Max
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// SafeRecorder обёртка над audit.Recorder, которая никогда не возвращает
// ошибку наружу: только логирует. Используется в инструментированных
// usecase'ах (orders, menu, prompts), чтобы фейл записи лога не валил
// бизнес-операцию.
//
//	rec := usecase.NewSafeRecorder(repo, log)
//	rec.Record(ctx, audit.Entry{...}) // никогда не вернёт ошибку
type SafeRecorder struct {
	inner audit.Recorder
	log   *slog.Logger
}

// NewSafeRecorder оборачивает Recorder.
func NewSafeRecorder(inner audit.Recorder, log *slog.Logger) *SafeRecorder {
	return &SafeRecorder{inner: inner, log: log}
}

// Record выполняет запись best-effort. Ошибка только логируется.
func (s *SafeRecorder) Record(ctx context.Context, e audit.Entry) {
	if s == nil || s.inner == nil {
		return
	}
	if err := s.inner.Record(ctx, e); err != nil {
		if s.log != nil {
			s.log.Warn("admin action audit failed",
				"target", string(e.Target),
				"target_id", e.TargetID,
				"verb", string(e.Verb),
				"err", err,
			)
		}
	}
}
