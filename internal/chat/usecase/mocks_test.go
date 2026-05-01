package usecase

import (
	"context"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
)

// mockRepo ручной мок chat.Repository с func-полями. nil-поле → panic в тесте,
// что чётко обозначает: тест задействует метод, не подразумевавшийся сценарием.
type mockRepo struct {
	createChatFn           func(ctx context.Context, c *repositorymodels.Chat) error
	findChatByIDFn         func(ctx context.Context, id uuid.UUID) (*repositorymodels.Chat, error)
	findMostRecentByUserFn func(ctx context.Context, userID uuid.UUID) (*repositorymodels.Chat, error)
	listChatsByUserFn      func(ctx context.Context, userID uuid.UUID, limit, offset int) ([]repositorymodels.Chat, int, error)
	deleteChatFn           func(ctx context.Context, id uuid.UUID) error
	appendMessageFn        func(ctx context.Context, m *repositorymodels.Message) error
	listMessagesFn         func(ctx context.Context, chatID uuid.UUID, limit int, before *uuid.UUID) ([]repositorymodels.Message, bool, error)
	findFirstUserMessageFn func(ctx context.Context, chatID, excludeID uuid.UUID) (*repositorymodels.Message, error)
}

func (m *mockRepo) CreateChat(ctx context.Context, c *repositorymodels.Chat) error {
	return m.createChatFn(ctx, c)
}

func (m *mockRepo) FindChatByID(ctx context.Context, id uuid.UUID) (*repositorymodels.Chat, error) {
	return m.findChatByIDFn(ctx, id)
}

func (m *mockRepo) FindMostRecentByUser(ctx context.Context, userID uuid.UUID) (*repositorymodels.Chat, error) {
	return m.findMostRecentByUserFn(ctx, userID)
}

func (m *mockRepo) ListChatsByUser(
	ctx context.Context, userID uuid.UUID, limit, offset int,
) ([]repositorymodels.Chat, int, error) {
	return m.listChatsByUserFn(ctx, userID, limit, offset)
}

func (m *mockRepo) DeleteChat(ctx context.Context, id uuid.UUID) error {
	return m.deleteChatFn(ctx, id)
}

func (m *mockRepo) AppendMessage(ctx context.Context, msg *repositorymodels.Message) error {
	return m.appendMessageFn(ctx, msg)
}

func (m *mockRepo) ListMessages(
	ctx context.Context, chatID uuid.UUID, limit int, before *uuid.UUID,
) ([]repositorymodels.Message, bool, error) {
	return m.listMessagesFn(ctx, chatID, limit, before)
}

func (m *mockRepo) FindFirstUserMessage(
	ctx context.Context, chatID, excludeID uuid.UUID,
) (*repositorymodels.Message, error) {
	if m.findFirstUserMessageFn == nil {
		return nil, nil
	}
	return m.findFirstUserMessageFn(ctx, chatID, excludeID)
}

// mockUUID детерминированный генератор UUID; next выдаётся первым, потом nextN[0..]
type mockUUID struct {
	next  uuid.UUID
	nextN []uuid.UUID
	calls int
}

func (m *mockUUID) New() uuid.UUID {
	defer func() { m.calls++ }()
	if m.calls == 0 {
		return m.next
	}
	idx := m.calls - 1
	if idx < len(m.nextN) {
		return m.nextN[idx]
	}
	// Если тест не предусмотрел дополнительные UUID — возвращаем нулевой,
	// тест должен это заметить.
	return uuid.Nil
}
