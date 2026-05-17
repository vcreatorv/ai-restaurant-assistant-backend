// Package usecase реализует suggestions.Usecase.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/suggestions"
)

type usecaseImpl struct {
	repo suggestions.Repository
}

// New создаёт suggestions.Usecase.
func New(repo suggestions.Repository) suggestions.Usecase {
	return &usecaseImpl{repo: repo}
}

// ListActive возвращает только активные подсказки для публичного API чата.
func (u *usecaseImpl) ListActive(ctx context.Context) ([]usecasemodels.ChatSuggestion, error) {
	full, err := u.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active suggestions: %w", err)
	}
	out := make([]usecasemodels.ChatSuggestion, len(full))
	for i, s := range full {
		out[i] = usecasemodels.ChatSuggestion{ID: s.ID, Text: s.Text}
	}
	return out, nil
}

// ListAll возвращает все подсказки для админки.
func (u *usecaseImpl) ListAll(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error) {
	return u.repo.ListAll(ctx)
}

// Create создаёт подсказку с валидацией длины текста.
func (u *usecaseImpl) Create(
	ctx context.Context,
	c usecasemodels.ChatSuggestionCreate,
) (*usecasemodels.AdminChatSuggestion, error) {
	if !isValidText(c.Text) {
		return nil, suggestions.ErrInvalidText
	}
	s := &usecasemodels.AdminChatSuggestion{
		Text:      c.Text,
		SortOrder: c.SortOrder,
		IsActive:  c.IsActive,
	}
	if err := u.repo.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Update применяет частичный патч к подсказке.
func (u *usecaseImpl) Update(
	ctx context.Context,
	id int64,
	p usecasemodels.ChatSuggestionPatch,
) (*usecasemodels.AdminChatSuggestion, error) {
	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Text != nil {
		if !isValidText(*p.Text) {
			return nil, suggestions.ErrInvalidText
		}
		existing.Text = *p.Text
	}
	if p.SortOrder != nil {
		existing.SortOrder = *p.SortOrder
	}
	if p.IsActive != nil {
		existing.IsActive = *p.IsActive
	}
	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete удаляет подсказку.
func (u *usecaseImpl) Delete(ctx context.Context, id int64) error {
	return u.repo.Delete(ctx, id)
}

// RegisterClick инкрементирует счётчик. Несуществующий id не возвращает ошибку клиенту —
// это просто промах аналитики, не функциональная проблема.
func (u *usecaseImpl) RegisterClick(ctx context.Context, id int64) error {
	if err := u.repo.IncrementClicks(ctx, id); err != nil {
		if errors.Is(err, suggestions.ErrNotFound) {
			logger.ForCtx(ctx).Debug("suggestion click for missing id", "id", id)
			return nil
		}
		return err
	}
	return nil
}

func isValidText(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= suggestions.TextMinLen && n <= suggestions.TextMaxLen
}
