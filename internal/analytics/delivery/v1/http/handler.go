// Package http реализует HTTP-делирий для админ-аналитики.
package http

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/analytics"
	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// AnalyticsHandler HTTP-делирий для админ-аналитики.
type AnalyticsHandler struct {
	usecase analytics.Usecase
	users   user.Usecase
}

// New создаёт AnalyticsHandler.
func New(uc analytics.Usecase, users user.Usecase) AnalyticsHandler {
	return AnalyticsHandler{usecase: uc, users: users}
}

func (h AnalyticsHandler) requireAdmin(ctx context.Context) error {
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

// AdminGetDashboard реализует GET /admin/dashboard.
func (h AnalyticsHandler) AdminGetDashboard(
	ctx context.Context,
	request v1.AdminGetDashboardRequestObject,
) (v1.AdminGetDashboardResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	period := analytics.Period(request.Params.Period)
	if !period.Valid() {
		return nil, apperrors.ErrBadRequest
	}
	d, err := h.usecase.GetDashboard(ctx, period)
	if err != nil {
		return nil, err
	}
	return v1.AdminGetDashboard200JSONResponse(apimodels.AdminDashboardFromUsecase(*d)), nil
}

// AdminGetAnalytics реализует GET /admin/analytics.
func (h AnalyticsHandler) AdminGetAnalytics(
	ctx context.Context,
	request v1.AdminGetAnalyticsRequestObject,
) (v1.AdminGetAnalyticsResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	period := analytics.Period(request.Params.Period)
	if !period.Valid() {
		return nil, apperrors.ErrBadRequest
	}
	a, err := h.usecase.GetAnalytics(ctx, period)
	if err != nil {
		return nil, err
	}
	return v1.AdminGetAnalytics200JSONResponse(apimodels.AdminAnalyticsFromUsecase(*a)), nil
}
