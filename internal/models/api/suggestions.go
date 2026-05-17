package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// ChatSuggestionFromUsecase маппит public-вид подсказки.
func ChatSuggestionFromUsecase(s usecasemodels.ChatSuggestion) ChatSuggestion {
	return ChatSuggestion{
		Id:   safeInt64ToInt(s.ID),
		Text: s.Text,
	}
}

// ChatSuggestionListFromUsecase маппит slice public-вида.
func ChatSuggestionListFromUsecase(ss []usecasemodels.ChatSuggestion) ChatSuggestionList {
	items := make([]ChatSuggestion, len(ss))
	for i, s := range ss {
		items[i] = ChatSuggestionFromUsecase(s)
	}
	return ChatSuggestionList{Items: items}
}

// AdminChatSuggestionFromUsecase маппит админский вид.
func AdminChatSuggestionFromUsecase(s usecasemodels.AdminChatSuggestion) AdminChatSuggestion {
	return AdminChatSuggestion{
		Id:           safeInt64ToInt(s.ID),
		Text:         s.Text,
		SortOrder:    s.SortOrder,
		IsActive:     s.IsActive,
		ClicksCount:  s.ClicksCount,
	}
}

// AdminChatSuggestionListFromUsecase маппит slice админского вида.
func AdminChatSuggestionListFromUsecase(ss []usecasemodels.AdminChatSuggestion) AdminChatSuggestionList {
	items := make([]AdminChatSuggestion, len(ss))
	for i, s := range ss {
		items[i] = AdminChatSuggestionFromUsecase(s)
	}
	return AdminChatSuggestionList{Items: items}
}

// CreateChatSuggestionRequestToUsecase маппит API → usecase Create.
func CreateChatSuggestionRequestToUsecase(req CreateChatSuggestionRequest) usecasemodels.ChatSuggestionCreate {
	c := usecasemodels.ChatSuggestionCreate{
		Text:      req.Text,
		SortOrder: 0,
		IsActive:  true,
	}
	if req.SortOrder != nil {
		c.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	return c
}

// PatchChatSuggestionRequestToUsecase маппит API → usecase Patch.
func PatchChatSuggestionRequestToUsecase(req PatchChatSuggestionRequest) usecasemodels.ChatSuggestionPatch {
	return usecasemodels.ChatSuggestionPatch{
		Text:      req.Text,
		SortOrder: req.SortOrder,
		IsActive:  req.IsActive,
	}
}

// safeInt64ToInt сужает id из БД (bigserial) в int для API. На практике id <= 2^53.
func safeInt64ToInt(v int64) int {
	return int(v)
}
