package app

import (
	authdelivery "github.com/example/ai-restaurant-assistant-backend/internal/auth/delivery/v1/http"
	menudelivery "github.com/example/ai-restaurant-assistant-backend/internal/menu/delivery/v1/http"
	userdelivery "github.com/example/ai-restaurant-assistant-backend/internal/user/delivery/v1/http"
)

// Handler корневой handler, реализующий v1.StrictServerInterface через embedding
type Handler struct {
	authdelivery.AuthHandler
	userdelivery.UserHandler
	menudelivery.MenuHandler
	Unimplemented
}
