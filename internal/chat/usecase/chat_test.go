package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func userID() uuid.UUID    { return uuid.MustParse("11111111-1111-1111-1111-111111111111") }
func chatID() uuid.UUID    { return uuid.MustParse("22222222-2222-2222-2222-222222222222") }
func newChatID() uuid.UUID { return uuid.MustParse("33333333-3333-3333-3333-333333333333") }
func msgAsstID() uuid.UUID { return uuid.MustParse("55555555-5555-5555-5555-555555555555") }

const ttl = 6 * time.Hour

// newUC собирает chat.Usecase из mockRepo + mockUUID для unit-тестов
// без RAG-зависимостей (Cohere/Qdrant/LLM). Сценарии, не доходящие до RAG,
// оставляют эти поля nil.
func newUC(repo chat.Repository, uuidGen chat.UUIDGen, cfg chat.UsecaseConfig) chat.Usecase {
	return New(Deps{Repo: repo, UUID: uuidGen, ChatCfg: cfg})
}

// ----- GetActive -----

func TestGetActive_FreshChat_Returned(t *testing.T) {
	fresh := time.Now().Add(-time.Hour)
	repo := &mockRepo{
		findMostRecentByUserFn: func(_ context.Context, uid uuid.UUID) (*repositorymodels.Chat, error) {
			require.Equal(t, userID(), uid)
			return &repositorymodels.Chat{ID: chatID(), UserID: uid, LastMessageAt: fresh}, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{AutoNewChatAfter: ttl})

	c, err := uc.GetActive(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, chatID(), c.ID)
}

func TestGetActive_StaleChat_NewCreated(t *testing.T) {
	stale := time.Now().Add(-12 * time.Hour)
	createdNew := false
	repo := &mockRepo{
		findMostRecentByUserFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: userID(), LastMessageAt: stale}, nil
		},
		createChatFn: func(_ context.Context, c *repositorymodels.Chat) error {
			require.Equal(t, newChatID(), c.ID, "новый id из uuid generator")
			require.Equal(t, userID(), c.UserID)
			createdNew = true
			return nil
		},
	}
	uc := newUC(repo, &mockUUID{next: newChatID()}, chat.UsecaseConfig{AutoNewChatAfter: ttl})

	c, err := uc.GetActive(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, newChatID(), c.ID)
	require.True(t, createdNew, "stale → создаём новый")
}

func TestGetActive_NoChats_NewCreated(t *testing.T) {
	repo := &mockRepo{
		findMostRecentByUserFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return nil, chat.ErrChatNotFound
		},
		createChatFn: func(_ context.Context, c *repositorymodels.Chat) error {
			require.Equal(t, newChatID(), c.ID)
			return nil
		},
	}
	uc := newUC(repo, &mockUUID{next: newChatID()}, chat.UsecaseConfig{AutoNewChatAfter: ttl})

	c, err := uc.GetActive(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, newChatID(), c.ID)
}

func TestGetActive_TTLZero_AlwaysReuses(t *testing.T) {
	veryOld := time.Now().Add(-30 * 24 * time.Hour)
	repo := &mockRepo{
		findMostRecentByUserFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: userID(), LastMessageAt: veryOld}, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{AutoNewChatAfter: 0})

	c, err := uc.GetActive(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, chatID(), c.ID, "при ttl=0 stale-проверка отключена")
}

// ----- Create -----

func TestCreate_HappyPath(t *testing.T) {
	title := "обед"
	repo := &mockRepo{
		createChatFn: func(_ context.Context, c *repositorymodels.Chat) error {
			require.Equal(t, newChatID(), c.ID)
			require.Equal(t, userID(), c.UserID)
			require.Equal(t, &title, c.Title)
			return nil
		},
	}
	uc := newUC(repo, &mockUUID{next: newChatID()}, chat.UsecaseConfig{})

	c, err := uc.Create(context.Background(), userID(), &title)
	require.NoError(t, err)
	require.Equal(t, "обед", c.Title)
}

// ----- GetWithMessages -----

func TestGetWithMessages_HappyPath_OrdersASC(t *testing.T) {
	now := time.Now()
	repo := &mockRepo{
		findChatByIDFn: func(_ context.Context, id uuid.UUID) (*repositorymodels.Chat, error) {
			require.Equal(t, chatID(), id)
			return &repositorymodels.Chat{ID: chatID(), UserID: userID()}, nil
		},
		listMessagesFn: func(
			_ context.Context, cid uuid.UUID, _ int, _ *uuid.UUID,
		) ([]repositorymodels.Message, bool, error) {
			return []repositorymodels.Message{
				{ID: msgAsstID(), ChatID: cid, Role: "assistant", Content: "ответ", CreatedAt: now},
				{ID: uuid.New(), ChatID: cid, Role: "user", Content: "вопрос", CreatedAt: now.Add(-time.Minute)},
			}, false, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})

	_, msgs, hasMore, err := uc.GetWithMessages(context.Background(), userID(), chatID(), 50, nil)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, msgs, 2)
	require.Equal(t, "вопрос", msgs[0].Content, "ASC: старые сначала")
	require.Equal(t, "ответ", msgs[1].Content)
}

func TestGetWithMessages_ChatNotOwned_Forbidden(t *testing.T) {
	otherUser := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	repo := &mockRepo{
		findChatByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: otherUser}, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})

	_, _, _, err := uc.GetWithMessages(context.Background(), userID(), chatID(), 50, nil)
	require.ErrorIs(t, err, chat.ErrChatForbidden)
}

func TestGetWithMessages_ChatNotFound(t *testing.T) {
	repo := &mockRepo{
		findChatByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return nil, chat.ErrChatNotFound
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})

	_, _, _, err := uc.GetWithMessages(context.Background(), userID(), chatID(), 50, nil)
	require.ErrorIs(t, err, chat.ErrChatNotFound)
}

// ----- Delete -----

func TestDelete_NotOwned_Forbidden(t *testing.T) {
	other := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	repo := &mockRepo{
		findChatByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: other}, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})
	require.ErrorIs(t, uc.Delete(context.Background(), userID(), chatID()), chat.ErrChatForbidden)
}

func TestDelete_HappyPath(t *testing.T) {
	deleted := false
	repo := &mockRepo{
		findChatByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: userID()}, nil
		},
		deleteChatFn: func(_ context.Context, id uuid.UUID) error {
			require.Equal(t, chatID(), id)
			deleted = true
			return nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})
	require.NoError(t, uc.Delete(context.Background(), userID(), chatID()))
	require.True(t, deleted)
}

// ----- SendMessage (edge-cases без RAG) -----
//
// Полноценные тесты pipeline (Cohere/Qdrant/OpenRouter) требуют моков внешних SDK
// и встанут в шаге A3.8 после стабилизации интерфейсов. Здесь — только защиты.

func TestSendMessage_EmptyContent_Rejected(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockUUID{}, chat.UsecaseConfig{})
	err := uc.SendMessage(context.Background(), userID(), chatID(), "   ", chat.SendCallbacks{})
	require.ErrorIs(t, err, chat.ErrEmptyMessage, "пустой контент с пробелами — не доходит до репо")
}

func TestSendMessage_ChatForbidden(t *testing.T) {
	other := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	repo := &mockRepo{
		findChatByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Chat, error) {
			return &repositorymodels.Chat{ID: chatID(), UserID: other}, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})
	err := uc.SendMessage(context.Background(), userID(), chatID(), "hi", chat.SendCallbacks{})
	require.ErrorIs(t, err, chat.ErrChatForbidden)
}

// ----- List -----

func TestList_PassesPagingThrough(t *testing.T) {
	repo := &mockRepo{
		listChatsByUserFn: func(_ context.Context, uid uuid.UUID, limit, offset int) ([]repositorymodels.Chat, int, error) {
			require.Equal(t, userID(), uid)
			require.Equal(t, 25, limit)
			require.Equal(t, 50, offset)
			return []repositorymodels.Chat{{ID: chatID(), UserID: uid}}, 100, nil
		},
	}
	uc := newUC(repo, &mockUUID{}, chat.UsecaseConfig{})

	items, total, err := uc.List(context.Background(), userID(), 25, 50)
	require.NoError(t, err)
	require.Equal(t, 100, total)
	require.Len(t, items, 1)
}

// ----- parseLLMTail -----

func TestParseLLMTail_ExtractsAndStrips(t *testing.T) {
	raw := "Рекомендую попробовать Том-ям и Бабагануш.\n\n```json\n{\"recommended_dish_ids\":[32,1]}\n```"
	clean, ids := parseLLMTail(raw)
	require.Equal(t, "Рекомендую попробовать Том-ям и Бабагануш.", clean)
	require.Equal(t, []int{32, 1}, ids)
}

func TestParseLLMTail_NoJSON_ReturnsEmpty(t *testing.T) {
	raw := "Просто ответ без блока."
	clean, ids := parseLLMTail(raw)
	require.Equal(t, "Просто ответ без блока.", clean)
	require.Equal(t, []int{}, ids)
}
