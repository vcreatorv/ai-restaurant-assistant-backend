package repository

import (
	"time"

	"github.com/google/uuid"
)

// Chat чат пользователя в storage-форме
type Chat struct {
	// ID идентификатор чата
	ID uuid.UUID
	// UserID владелец чата
	UserID uuid.UUID
	// Title заголовок чата (опциональный)
	Title *string
	// LastMessageAt время последнего сообщения; используется для auto-new-chat
	LastMessageAt time.Time
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}

// Message сообщение чата в storage-форме
type Message struct {
	// ID идентификатор сообщения
	ID uuid.UUID
	// ChatID родительский чат
	ChatID uuid.UUID
	// Role роль автора: user, assistant, system
	Role string
	// Content тело сообщения
	Content string
	// RecommendedDishIDs идентификаторы блюд, которые ассистент порекомендовал в этом сообщении
	RecommendedDishIDs []int
	// Meta служебная мета (tokens_in/out, latency_ms, model и т.п.)
	Meta map[string]any
	// CreatedAt время создания
	CreatedAt time.Time
}
