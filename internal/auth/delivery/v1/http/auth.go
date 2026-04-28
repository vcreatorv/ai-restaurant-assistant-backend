package http

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
)

// GetSession реализует GET /auth/session
func (h AuthHandler) GetSession(
	ctx context.Context,
	_ v1.GetSessionRequestObject,
) (v1.GetSessionResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil {
		return nil, apperrors.ErrInternalNoSession
	}

	var u *usecasemodels.User
	if s.UserID != nil {
		var err error
		u, err = h.user.GetByID(ctx, *s.UserID)
		if err != nil {
			return nil, err
		}
	}
	return v1.GetSession200JSONResponse(apimodels.SessionInfoFromUsecase(s, u)), nil
}

// Register реализует POST /auth/register
func (h AuthHandler) Register(
	ctx context.Context,
	request v1.RegisterRequestObject,
) (v1.RegisterResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil {
		return nil, apperrors.ErrInternalNoSession
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}

	u, newSession, err := h.auth.Register(
		ctx, s.ID, s.UserID,
		string(request.Body.Email), request.Body.Password,
	)
	if err != nil {
		return nil, err
	}
	return v1.Register200JSONResponse(apimodels.SessionInfoFromUsecase(newSession, u)), nil
}

// Login реализует POST /auth/login
func (h AuthHandler) Login(
	ctx context.Context,
	request v1.LoginRequestObject,
) (v1.LoginResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil {
		return nil, apperrors.ErrInternalNoSession
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}

	u, newSession, err := h.auth.Login(
		ctx, s.ID,
		string(request.Body.Email), request.Body.Password,
	)
	if err != nil {
		return nil, err
	}
	return v1.Login200JSONResponse(apimodels.SessionInfoFromUsecase(newSession, u)), nil
}

// Logout реализует POST /auth/logout
func (h AuthHandler) Logout(
	ctx context.Context,
	_ v1.LogoutRequestObject,
) (v1.LogoutResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil {
		return nil, apperrors.ErrInternalNoSession
	}
	if err := h.auth.Logout(ctx, s.ID); err != nil {
		return nil, err
	}
	return v1.Logout204Response{}, nil
}

// ChangePassword реализует PATCH /auth/password
func (h AuthHandler) ChangePassword(
	ctx context.Context,
	request v1.ChangePasswordRequestObject,
) (v1.ChangePasswordResponseObject, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil {
		return nil, apperrors.ErrInternalNoSession
	}
	if s.UserID == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	if err := h.auth.ChangePassword(
		ctx, *s.UserID,
		request.Body.CurrentPassword, request.Body.NewPassword,
	); err != nil {
		return nil, err
	}
	return v1.ChangePassword204Response{}, nil
}
