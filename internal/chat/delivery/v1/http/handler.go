// Package http содержит HTTP-делирий чатов.
package http

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
)

// ChatHandler HTTP-делирий для chat-фичи
type ChatHandler struct {
	cfg     chat.DeliveryConfig
	usecase chat.Usecase
}

// New создаёт ChatHandler
func New(cfg chat.DeliveryConfig, uc chat.Usecase) ChatHandler {
	return ChatHandler{cfg: cfg, usecase: uc}
}
