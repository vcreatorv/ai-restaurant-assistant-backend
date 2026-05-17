package usecase

import "time"

// ChatSuggestion упрощённое представление подсказки для публичного GET /chat/suggestions.
// Гостям не нужны технические поля (sort_order, is_active, clicks_count).
type ChatSuggestion struct {
	// ID идентификатор подсказки
	ID int64
	// Text текст подсказки
	Text string
}

// AdminChatSuggestion полное представление подсказки для админки.
type AdminChatSuggestion struct {
	// ID идентификатор
	ID int64
	// Text текст подсказки (1..80 символов)
	Text string
	// SortOrder порядок отображения; меньше — выше
	SortOrder int
	// IsActive показывается ли гостям
	IsActive bool
	// ClicksCount сколько раз гости кликали по подсказке
	ClicksCount int64
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего изменения
	UpdatedAt time.Time
}

// ChatSuggestionCreate данные для создания подсказки.
type ChatSuggestionCreate struct {
	Text      string
	SortOrder int
	IsActive  bool
}

// ChatSuggestionPatch частичное обновление подсказки.
type ChatSuggestionPatch struct {
	Text      *string
	SortOrder *int
	IsActive  *bool
}
