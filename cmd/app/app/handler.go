package app

import (
	analyticsdelivery "github.com/example/ai-restaurant-assistant-backend/internal/analytics/delivery/v1/http"
	auditdelivery "github.com/example/ai-restaurant-assistant-backend/internal/audit/delivery/v1/http"
	authdelivery "github.com/example/ai-restaurant-assistant-backend/internal/auth/delivery/v1/http"
	cartdelivery "github.com/example/ai-restaurant-assistant-backend/internal/cart/delivery/v1/http"
	chatdelivery "github.com/example/ai-restaurant-assistant-backend/internal/chat/delivery/v1/http"
	menudelivery "github.com/example/ai-restaurant-assistant-backend/internal/menu/delivery/v1/http"
	orderdelivery "github.com/example/ai-restaurant-assistant-backend/internal/order/delivery/v1/http"
	promptsdelivery "github.com/example/ai-restaurant-assistant-backend/internal/prompts/delivery/v1/http"
	suggestionsdelivery "github.com/example/ai-restaurant-assistant-backend/internal/suggestions/delivery/v1/http"
	userdelivery "github.com/example/ai-restaurant-assistant-backend/internal/user/delivery/v1/http"
)

// Handler корневой handler, реализующий v1.StrictServerInterface через embedding.
// Конфликтов имён методов нет — каждый сабхэндлер покрывает свой набор endpoint'ов.
type Handler struct {
	authdelivery.AuthHandler
	userdelivery.UserHandler
	menudelivery.MenuHandler
	chatdelivery.ChatHandler
	cartdelivery.CartHandler
	orderdelivery.OrderHandler
	promptsdelivery.PromptsHandler
	auditdelivery.AuditHandler
	suggestionsdelivery.SuggestionsHandler
	analyticsdelivery.AnalyticsHandler
	Unimplemented
}
