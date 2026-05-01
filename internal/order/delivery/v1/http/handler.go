// Package http реализует HTTP delivery order-фичи поверх strict-server.
package http

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/order"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"
)

// OrderHandler HTTP-делирий для order-фичи
type OrderHandler struct {
	cfg     order.DeliveryConfig
	usecase order.Usecase
	users   user.Usecase
}

// New создаёт OrderHandler
func New(cfg order.DeliveryConfig, uc order.Usecase, users user.Usecase) OrderHandler {
	return OrderHandler{cfg: cfg, usecase: uc, users: users}
}

// requireUserID возвращает userID из контекстной сессии или ErrUnauthenticated
func (h OrderHandler) requireUserID(ctx context.Context) (uuid.UUID, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return uuid.Nil, apperrors.ErrUnauthenticated
	}
	return *s.UserID, nil
}

// requireAdmin проверяет, что текущая сессия принадлежит admin'у
func (h OrderHandler) requireAdmin(ctx context.Context) error {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return apperrors.ErrUnauthenticated
	}
	u, err := h.users.GetByID(ctx, *s.UserID)
	if err != nil {
		return err
	}
	if !u.IsAdmin() {
		return apperrors.ErrForbidden
	}
	return nil
}

// resolveListPaging применяет дефолты и кэп для GET /orders
func (h OrderHandler) resolveListPaging(limit, offset *int) (int, int) {
	l := h.cfg.ListDefaultLimit
	if limit != nil {
		l = *limit
	}
	if l <= 0 {
		l = h.cfg.ListDefaultLimit
	}
	if h.cfg.ListMaxLimit > 0 && l > h.cfg.ListMaxLimit {
		l = h.cfg.ListMaxLimit
	}
	o := 0
	if offset != nil && *offset > 0 {
		o = *offset
	}
	return l, o
}
