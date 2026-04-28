package http

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
)

// GetProfile реализует GET /profile
func (h UserHandler) GetProfile(
	ctx context.Context,
	_ v1.GetProfileRequestObject,
) (v1.GetProfileResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	u, err := h.usecase.GetProfile(ctx, *s.UserID)
	if err != nil {
		return nil, err
	}
	return v1.GetProfile200JSONResponse(apimodels.ProfileFromUsecase(u)), nil
}

// UpdateProfile реализует PATCH /profile
func (h UserHandler) UpdateProfile(
	ctx context.Context,
	request v1.UpdateProfileRequestObject,
) (v1.UpdateProfileResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	patch := apimodels.PatchProfileRequestToUsecase(*request.Body)
	u, err := h.usecase.UpdateProfile(ctx, *s.UserID, patch)
	if err != nil {
		return nil, err
	}
	return v1.UpdateProfile200JSONResponse(apimodels.ProfileFromUsecase(u)), nil
}
