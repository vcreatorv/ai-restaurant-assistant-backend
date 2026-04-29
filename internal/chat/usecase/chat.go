package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

// GetActive возвращает активный чат пользователя; создаёт новый, если последний устарел или отсутствует
func (uc *chatUsecase) GetActive(ctx context.Context, userID uuid.UUID) (*usecasemodels.Chat, error) {
	raw, err := uc.repo.FindMostRecentByUser(ctx, userID)
	if err == nil {
		if !uc.isStale(raw.LastMessageAt) {
			return usecasemodels.ChatFromRepository(raw), nil
		}
	} else if !errors.Is(err, chat.ErrChatNotFound) {
		return nil, fmt.Errorf("find recent chat: %w", err)
	}
	return uc.Create(ctx, userID, nil)
}

// List возвращает чаты пользователя
func (uc *chatUsecase) List(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]usecasemodels.Chat, int, error) {
	items, total, err := uc.repo.ListChatsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list chats: %w", err)
	}
	return usecasemodels.ChatsFromRepository(items), total, nil
}

// Create создаёт чат с опциональным заголовком
func (uc *chatUsecase) Create(
	ctx context.Context,
	userID uuid.UUID,
	title *string,
) (*usecasemodels.Chat, error) {
	c := &repositorymodels.Chat{
		ID:     uc.uuid.New(),
		UserID: userID,
		Title:  title,
	}
	if err := uc.repo.CreateChat(ctx, c); err != nil {
		return nil, fmt.Errorf("create chat: %w", err)
	}
	return usecasemodels.ChatFromRepository(c), nil
}

// GetWithMessages возвращает чат и сообщения; before — курсор
func (uc *chatUsecase) GetWithMessages(
	ctx context.Context,
	userID, chatID uuid.UUID,
	limit int,
	before *uuid.UUID,
) (*usecasemodels.Chat, []usecasemodels.Message, bool, error) {
	c, err := uc.loadChatOwned(ctx, userID, chatID)
	if err != nil {
		return nil, nil, false, err
	}
	rawMessages, hasMore, err := uc.repo.ListMessages(ctx, chatID, limit, before)
	if err != nil {
		return nil, nil, false, fmt.Errorf("list messages: %w", err)
	}
	// История запрашивается DESC, отдаём ASC (старые → новые) — это удобнее UI.
	reverseMessages(rawMessages)
	return usecasemodels.ChatFromRepository(c), usecasemodels.MessagesFromRepository(rawMessages), hasMore, nil
}

// Delete удаляет чат пользователя
func (uc *chatUsecase) Delete(ctx context.Context, userID, chatID uuid.UUID) error {
	if _, err := uc.loadChatOwned(ctx, userID, chatID); err != nil {
		return err
	}
	return uc.repo.DeleteChat(ctx, chatID)
}

// SendMessage сохраняет сообщение пользователя и эхо-ответ ассистента (заглушка для A1)
func (uc *chatUsecase) SendMessage(
	ctx context.Context,
	userID, chatID uuid.UUID,
	content string,
) (*usecasemodels.Message, *usecasemodels.Message, error) {
	if strings.TrimSpace(content) == "" {
		return nil, nil, chat.ErrEmptyMessage
	}
	if _, err := uc.loadChatOwned(ctx, userID, chatID); err != nil {
		return nil, nil, err
	}

	userMsg := &repositorymodels.Message{
		ID:                 uc.uuid.New(),
		ChatID:             chatID,
		Role:               string(usecasemodels.RoleUser),
		Content:            content,
		RecommendedDishIDs: []int{},
		Meta:               map[string]any{},
	}
	if err := uc.repo.AppendMessage(ctx, userMsg); err != nil {
		return nil, nil, fmt.Errorf("append user message: %w", err)
	}

	assistantMsg := &repositorymodels.Message{
		ID:                 uc.uuid.New(),
		ChatID:             chatID,
		Role:               string(usecasemodels.RoleAssistant),
		Content:            echoReply(content),
		RecommendedDishIDs: []int{},
		Meta:               map[string]any{"echo": true},
	}
	if err := uc.repo.AppendMessage(ctx, assistantMsg); err != nil {
		return nil, nil, fmt.Errorf("append assistant message: %w", err)
	}

	return usecasemodels.MessageFromRepository(userMsg), usecasemodels.MessageFromRepository(assistantMsg), nil
}

// loadChatOwned возвращает чат, если он принадлежит пользователю
func (uc *chatUsecase) loadChatOwned(
	ctx context.Context,
	userID, chatID uuid.UUID,
) (*repositorymodels.Chat, error) {
	c, err := uc.repo.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("find chat: %w", err)
	}
	if c.UserID != userID {
		return nil, chat.ErrChatForbidden
	}
	return c, nil
}

// isStale проверяет, что чат с таким last_message_at пора заменить новым
func (uc *chatUsecase) isStale(lastMessageAt time.Time) bool {
	if uc.cfg.AutoNewChatAfter <= 0 {
		return false
	}
	return time.Since(lastMessageAt) > uc.cfg.AutoNewChatAfter
}

// echoReply формирует текст эхо-ответа на сообщение пользователя
func echoReply(content string) string {
	return "echo: " + content
}

// reverseMessages переворачивает срез сообщений на месте
func reverseMessages(msgs []repositorymodels.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}
