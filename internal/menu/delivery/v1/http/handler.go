package http

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// MenuHandler HTTP-делирий для menu-фичи
type MenuHandler struct {
	cfg     menu.DeliveryConfig
	usecase menu.Usecase
	users   user.Usecase
}

// New создаёт MenuHandler
func New(cfg menu.DeliveryConfig, uc menu.Usecase, users user.Usecase) MenuHandler {
	return MenuHandler{cfg: cfg, usecase: uc, users: users}
}

// requireAdmin проверяет, что текущая сессия принадлежит admin'у
func (h MenuHandler) requireAdmin(ctx context.Context) error {
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
