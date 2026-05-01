// Package http реализует HTTP delivery cart-фичи поверх strict-server.
package http

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"

	"github.com/google/uuid"
)

// CartHandler HTTP-делирий для cart-фичи
type CartHandler struct {
	cfg     cart.DeliveryConfig
	usecase cart.Usecase
}

// New создаёт CartHandler
func New(cfg cart.DeliveryConfig, uc cart.Usecase) CartHandler {
	return CartHandler{cfg: cfg, usecase: uc}
}

// requireUserID возвращает userID из контекстной сессии или ErrUnauthenticated
func (h CartHandler) requireUserID(ctx context.Context) (uuid.UUID, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return uuid.Nil, apperrors.ErrUnauthenticated
	}
	return *s.UserID, nil
}
