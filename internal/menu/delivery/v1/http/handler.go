package http

import (
	"context"

	auditusecase "github.com/example/ai-restaurant-assistant-backend/internal/audit/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"
)

// MenuHandler HTTP-делирий для menu-фичи
type MenuHandler struct {
	cfg     menu.DeliveryConfig
	usecase menu.Usecase
	users   user.Usecase
	audit   *auditusecase.SafeRecorder
}

// New создаёт MenuHandler. audit может быть nil — тогда лог не пишется.
func New(cfg menu.DeliveryConfig, uc menu.Usecase, users user.Usecase, rec *auditusecase.SafeRecorder) MenuHandler {
	return MenuHandler{cfg: cfg, usecase: uc, users: users, audit: rec}
}

// requireAdmin проверяет, что текущая сессия принадлежит admin'у
func (h MenuHandler) requireAdmin(ctx context.Context) error {
	_, err := h.requireAdminID(ctx)
	return err
}

// requireAdminID — то же, что requireAdmin, но возвращает id админа.
func (h MenuHandler) requireAdminID(ctx context.Context) (uuid.UUID, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return uuid.Nil, apperrors.ErrUnauthenticated
	}
	u, err := h.users.GetByID(ctx, *s.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	if !u.IsAdmin() {
		return uuid.Nil, apperrors.ErrForbidden
	}
	return *s.UserID, nil
}

// fillListDishesDefaults заполняет дефолты и применяет верхнюю границу limit
func (h MenuHandler) fillListDishesDefaults(p *apimodels.ListDishesParams) {
	if p.Limit == nil || *p.Limit <= 0 {
		v := h.cfg.DefaultLimit
		p.Limit = &v
	}
	if *p.Limit > h.cfg.MaxLimit {
		v := h.cfg.MaxLimit
		p.Limit = &v
	}
	if p.Offset == nil || *p.Offset < 0 {
		v := 0
		p.Offset = &v
	}
}
