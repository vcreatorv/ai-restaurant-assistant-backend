package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// ChatFromUsecase маппит usecase-модель чата в api-DTO
func ChatFromUsecase(c *usecasemodels.Chat) Chat {
	out := Chat{
		Id:            c.ID,
		UserId:        c.UserID,
		LastMessageAt: c.LastMessageAt,
		CreatedAt:     c.CreatedAt,
	}
	if c.Title != "" {
		title := c.Title
		out.Title = &title
	}
	return out
}

// ChatListFromUsecase собирает api-DTO списка чатов
func ChatListFromUsecase(items []usecasemodels.Chat, total, limit, offset int) ChatList {
	out := ChatList{
		Items:  make([]Chat, 0, len(items)),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
	for i := range items {
		out.Items = append(out.Items, ChatFromUsecase(&items[i]))
	}
	return out
}

// MessageFromUsecase маппит usecase-модель сообщения в api-DTO
func MessageFromUsecase(m *usecasemodels.Message) Message {
	out := Message{
		Id:        m.ID,
		ChatId:    m.ChatID,
		Role:      MessageRole(m.Role),
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
	if len(m.RecommendedDishIDs) > 0 {
		ids := append([]int(nil), m.RecommendedDishIDs...)
		out.RecommendedDishIds = &ids
	}
	return out
}

// ChatWithMessagesFromUsecase собирает api-DTO чата с сообщениями
func ChatWithMessagesFromUsecase(
	chat *usecasemodels.Chat,
	messages []usecasemodels.Message,
	hasMore bool,
) ChatWithMessages {
	items := make([]Message, 0, len(messages))
	for i := range messages {
		items = append(items, MessageFromUsecase(&messages[i]))
	}
	hm := hasMore
	return ChatWithMessages{
		Chat:     ChatFromUsecase(chat),
		Messages: items,
		HasMore:  &hm,
	}
}
