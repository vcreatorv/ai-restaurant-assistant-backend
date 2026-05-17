// Package suggestions описывает доменные интерфейсы и ошибки фичи «подсказки чата».
package suggestions

import (
	"context"
	"errors"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

var (
	// ErrNotFound подсказка не найдена.
	ErrNotFound = errors.New("chat suggestion not found")
	// ErrInvalidText текст вне допустимого диапазона длины.
	ErrInvalidText = errors.New("invalid suggestion text")
)

// TextMinLen минимальная длина текста подсказки (символы).
const TextMinLen = 1

// TextMaxLen максимальная длина текста подсказки (символы).
const TextMaxLen = 80

// Usecase сценарии работы с подсказками чата.
type Usecase interface {
	// ListActive возвращает только активные подсказки, отсортированные по sort_order.
	// Используется публичным GET /chat/suggestions — для рендера чипов в чате.
	ListActive(ctx context.Context) ([]usecasemodels.ChatSuggestion, error)
	// ListAll возвращает все подсказки, включая неактивные. Только для админки.
	ListAll(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error)
	// Create создаёт новую подсказку.
	Create(ctx context.Context, c usecasemodels.ChatSuggestionCreate) (*usecasemodels.AdminChatSuggestion, error)
	// Update частично обновляет подсказку.
	Update(ctx context.Context, id int64, p usecasemodels.ChatSuggestionPatch) (*usecasemodels.AdminChatSuggestion, error)
	// Delete удаляет подсказку.
	Delete(ctx context.Context, id int64) error
	// RegisterClick инкрементирует счётчик кликов по подсказке (для аналитики).
	// Не возвращает ErrNotFound для несуществующих id — клиент не должен зависеть от состояния админки.
	RegisterClick(ctx context.Context, id int64) error
}

// Repository хранилище подсказок чата.
type Repository interface {
	ListActive(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error)
	ListAll(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error)
	FindByID(ctx context.Context, id int64) (*usecasemodels.AdminChatSuggestion, error)
	Create(ctx context.Context, c *usecasemodels.AdminChatSuggestion) error
	Update(ctx context.Context, c *usecasemodels.AdminChatSuggestion) error
	Delete(ctx context.Context, id int64) error
	IncrementClicks(ctx context.Context, id int64) error
}
