package usecase

import (
	"time"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
)

// MessageRole роль автора сообщения чата
type MessageRole string

const (
	// RoleUser сообщение от пользователя
	RoleUser MessageRole = "user"
	// RoleAssistant сообщение от ассистента
	RoleAssistant MessageRole = "assistant"
	// RoleSystem системное сообщение (не показывается пользователю)
	RoleSystem MessageRole = "system"
)

// IsValid проверяет, что роль допустима
func (r MessageRole) IsValid() bool {
	switch r {
	case RoleUser, RoleAssistant, RoleSystem:
		return true
	}
	return false
}

// Chat чат в доменной форме
type Chat struct {
	// ID идентификатор чата
	ID uuid.UUID
	// UserID владелец чата
	UserID uuid.UUID
	// Title заголовок чата
	Title string
	// LastMessageAt время последнего сообщения
	LastMessageAt time.Time
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}

// Message сообщение чата в доменной форме
type Message struct {
	// ID идентификатор сообщения
	ID uuid.UUID
	// ChatID родительский чат
	ChatID uuid.UUID
	// Role роль автора
	Role MessageRole
	// Content тело сообщения
	Content string
	// RecommendedDishIDs идентификаторы блюд, рекомендованных ассистентом
	RecommendedDishIDs []int
	// CreatedAt время создания
	CreatedAt time.Time
}

// ChatFromRepository маппит repository-модель в usecase-модель
func ChatFromRepository(r *repositorymodels.Chat) *Chat {
	if r == nil {
		return nil
	}
	return &Chat{
		ID:            r.ID,
		UserID:        r.UserID,
		Title:         derefString(r.Title),
		LastMessageAt: r.LastMessageAt,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

// ChatsFromRepository маппит список repository-моделей в usecase-модели
func ChatsFromRepository(rs []repositorymodels.Chat) []Chat {
	out := make([]Chat, 0, len(rs))
	for i := range rs {
		out = append(out, *ChatFromRepository(&rs[i]))
	}
	return out
}

// ChatToRepository маппит usecase-модель в repository-модель
func ChatToRepository(c *Chat) *repositorymodels.Chat {
	return &repositorymodels.Chat{
		ID:            c.ID,
		UserID:        c.UserID,
		Title:         nullableString(c.Title),
		LastMessageAt: c.LastMessageAt,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// MessageFromRepository маппит repository-модель в usecase-модель
func MessageFromRepository(r *repositorymodels.Message) *Message {
	if r == nil {
		return nil
	}
	dishes := r.RecommendedDishIDs
	if dishes == nil {
		dishes = []int{}
	}
	return &Message{
		ID:                 r.ID,
		ChatID:             r.ChatID,
		Role:               MessageRole(r.Role),
		Content:            r.Content,
		RecommendedDishIDs: dishes,
		CreatedAt:          r.CreatedAt,
	}
}

// MessagesFromRepository маппит список repository-моделей в usecase-модели
func MessagesFromRepository(rs []repositorymodels.Message) []Message {
	out := make([]Message, 0, len(rs))
	for i := range rs {
		out = append(out, *MessageFromRepository(&rs[i]))
	}
	return out
}
