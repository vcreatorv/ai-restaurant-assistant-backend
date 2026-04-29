// Package chat описывает доменные интерфейсы и ошибки чата ассистента.
package chat

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

var (
	// ErrChatNotFound чат не найден
	ErrChatNotFound = errors.New("chat not found")
	// ErrChatForbidden чат не принадлежит текущему пользователю
	ErrChatForbidden = errors.New("chat does not belong to user")
	// ErrEmptyMessage сообщение пустое
	ErrEmptyMessage = errors.New("message content is empty")
)

// Usecase сценарии работы с чатами
type Usecase interface {
	// GetActive возвращает активный чат пользователя; создаёт новый, если последний устарел
	GetActive(ctx context.Context, userID uuid.UUID) (*usecasemodels.Chat, error)
	// List возвращает чаты пользователя по убыванию last_message_at
	List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]usecasemodels.Chat, int, error)
	// Create создаёт новый чат с опциональным заголовком
	Create(ctx context.Context, userID uuid.UUID, title *string) (*usecasemodels.Chat, error)
	// GetWithMessages возвращает чат и последние сообщения; before — курсор по id
	GetWithMessages(
		ctx context.Context,
		userID, chatID uuid.UUID,
		limit int,
		before *uuid.UUID,
	) (*usecasemodels.Chat, []usecasemodels.Message, bool, error)
	// Delete удаляет чат пользователя
	Delete(ctx context.Context, userID, chatID uuid.UUID) error
	// SendMessage сохраняет user-message и возвращает ответ ассистента (на A1 — echo-заглушка)
	SendMessage(
		ctx context.Context,
		userID, chatID uuid.UUID,
		content string,
	) (userMsg, assistantMsg *usecasemodels.Message, err error)
}

// Repository хранилище чатов и сообщений
type Repository interface {
	// CreateChat вставляет чат
	CreateChat(ctx context.Context, c *repositorymodels.Chat) error
	// FindChatByID возвращает чат по идентификатору
	FindChatByID(ctx context.Context, id uuid.UUID) (*repositorymodels.Chat, error)
	// ListChatsByUser возвращает чаты пользователя по убыванию last_message_at + total
	ListChatsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]repositorymodels.Chat, int, error)
	// FindMostRecentByUser возвращает самый свежий чат пользователя или ErrChatNotFound
	FindMostRecentByUser(ctx context.Context, userID uuid.UUID) (*repositorymodels.Chat, error)
	// DeleteChat удаляет чат
	DeleteChat(ctx context.Context, id uuid.UUID) error

	// AppendMessage добавляет сообщение и обновляет last_message_at чата в одной транзакции
	AppendMessage(ctx context.Context, m *repositorymodels.Message) error
	// ListMessages возвращает сообщения чата (DESC по created_at), курсор before — id сообщения
	ListMessages(
		ctx context.Context,
		chatID uuid.UUID,
		limit int,
		before *uuid.UUID,
	) (msgs []repositorymodels.Message, hasMore bool, err error)
}

// UUIDGen генератор UUID
type UUIDGen interface {
	// New генерирует новый UUID
	New() uuid.UUID
}
