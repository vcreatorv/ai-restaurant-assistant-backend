// Package http реализует HTTP-делирий для подсказок чата.
package http

import (
	"context"

	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/suggestions"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// SuggestionsHandler HTTP-делирий для chat suggestions.
type SuggestionsHandler struct {
	usecase suggestions.Usecase
	users   user.Usecase
}

// New создаёт SuggestionsHandler.
func New(uc suggestions.Usecase, users user.Usecase) SuggestionsHandler {
	return SuggestionsHandler{usecase: uc, users: users}
}

// requireAdmin проверяет, что текущая сессия — admin.
func (h SuggestionsHandler) requireAdmin(ctx context.Context) error {
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

// ListChatSuggestions реализует GET /chat/suggestions (публичный).
func (h SuggestionsHandler) ListChatSuggestions(
	ctx context.Context,
	_ v1.ListChatSuggestionsRequestObject,
) (v1.ListChatSuggestionsResponseObject, error) {
	ss, err := h.usecase.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return v1.ListChatSuggestions200JSONResponse(apimodels.ChatSuggestionListFromUsecase(ss)), nil
}

// TrackChatSuggestionClick реализует POST /chat/suggestions/{id}/click.
func (h SuggestionsHandler) TrackChatSuggestionClick(
	ctx context.Context,
	request v1.TrackChatSuggestionClickRequestObject,
) (v1.TrackChatSuggestionClickResponseObject, error) {
	if err := h.usecase.RegisterClick(ctx, int64(request.Id)); err != nil {
		return nil, err
	}
	return v1.TrackChatSuggestionClick204Response{}, nil
}

// AdminListChatSuggestions реализует GET /admin/suggestions.
func (h SuggestionsHandler) AdminListChatSuggestions(
	ctx context.Context,
	_ v1.AdminListChatSuggestionsRequestObject,
) (v1.AdminListChatSuggestionsResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	ss, err := h.usecase.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return v1.AdminListChatSuggestions200JSONResponse(apimodels.AdminChatSuggestionListFromUsecase(ss)), nil
}

// AdminCreateChatSuggestion реализует POST /admin/suggestions.
func (h SuggestionsHandler) AdminCreateChatSuggestion(
	ctx context.Context,
	request v1.AdminCreateChatSuggestionRequestObject,
) (v1.AdminCreateChatSuggestionResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	s, err := h.usecase.Create(ctx, apimodels.CreateChatSuggestionRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminCreateChatSuggestion201JSONResponse(apimodels.AdminChatSuggestionFromUsecase(*s)), nil
}

// AdminUpdateChatSuggestion реализует PATCH /admin/suggestions/{id}.
func (h SuggestionsHandler) AdminUpdateChatSuggestion(
	ctx context.Context,
	request v1.AdminUpdateChatSuggestionRequestObject,
) (v1.AdminUpdateChatSuggestionResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	s, err := h.usecase.Update(
		ctx,
		int64(request.Id),
		apimodels.PatchChatSuggestionRequestToUsecase(*request.Body),
	)
	if err != nil {
		return nil, err
	}
	return v1.AdminUpdateChatSuggestion200JSONResponse(apimodels.AdminChatSuggestionFromUsecase(*s)), nil
}

// AdminDeleteChatSuggestion реализует DELETE /admin/suggestions/{id}.
func (h SuggestionsHandler) AdminDeleteChatSuggestion(
	ctx context.Context,
	request v1.AdminDeleteChatSuggestionRequestObject,
) (v1.AdminDeleteChatSuggestionResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.usecase.Delete(ctx, int64(request.Id)); err != nil {
		return nil, err
	}
	return v1.AdminDeleteChatSuggestion204Response{}, nil
}
