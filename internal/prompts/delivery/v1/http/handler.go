// Package http — HTTP-делирий промптов: маршруты /admin/prompts/*.
package http

import (
	"context"
	"fmt"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	auditusecase "github.com/example/ai-restaurant-assistant-backend/internal/audit/usecase"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"
)

// PromptsHandler HTTP-делирий промптов.
type PromptsHandler struct {
	usecase prompts.Usecase
	users   user.Usecase
	audit   *auditusecase.SafeRecorder
}

// New создаёт PromptsHandler. audit может быть nil — тогда лог не пишется.
func New(uc prompts.Usecase, users user.Usecase, rec *auditusecase.SafeRecorder) PromptsHandler {
	return PromptsHandler{usecase: uc, users: users, audit: rec}
}

// requireAdmin проверяет, что текущая сессия принадлежит админу, и возвращает его id.
func (h PromptsHandler) requireAdmin(ctx context.Context) (uuid.UUID, error) {
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

// AdminListPrompts реализует GET /admin/prompts.
func (h PromptsHandler) AdminListPrompts(
	ctx context.Context,
	_ v1.AdminListPromptsRequestObject,
) (v1.AdminListPromptsResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.usecase.List(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return v1.AdminListPrompts200JSONResponse(apimodels.PromptListFromUsecase(items)), nil
}

// AdminGetPrompt реализует GET /admin/prompts/details?name=<name>.
func (h PromptsHandler) AdminGetPrompt(
	ctx context.Context,
	request v1.AdminGetPromptRequestObject,
) (v1.AdminGetPromptResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.usecase.Get(ctx, prompts.Name(request.Params.Name), adminID)
	if err != nil {
		return nil, err
	}
	return v1.AdminGetPrompt200JSONResponse(apimodels.PromptDetailsFromUsecase(*d)), nil
}

// AdminUpsertPromptDraft реализует PUT /admin/prompts/draft?name=<name>.
func (h PromptsHandler) AdminUpsertPromptDraft(
	ctx context.Context,
	request v1.AdminUpsertPromptDraftRequestObject,
) (v1.AdminUpsertPromptDraftResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	d, err := h.usecase.UpsertDraft(ctx, prompts.Name(request.Params.Name), adminID, request.Body.Content)
	if err != nil {
		return nil, err
	}
	return v1.AdminUpsertPromptDraft200JSONResponse(*apimodels.PromptDraftFromUsecase(d)), nil
}

// AdminDeletePromptDraft реализует DELETE /admin/prompts/draft?name=<name>.
func (h PromptsHandler) AdminDeletePromptDraft(
	ctx context.Context,
	request v1.AdminDeletePromptDraftRequestObject,
) (v1.AdminDeletePromptDraftResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.DeleteDraft(ctx, prompts.Name(request.Params.Name), adminID); err != nil {
		return nil, err
	}
	return v1.AdminDeletePromptDraft204Response{}, nil
}

// AdminPublishPrompt реализует POST /admin/prompts/publish?name=<name>.
func (h PromptsHandler) AdminPublishPrompt(
	ctx context.Context,
	request v1.AdminPublishPromptRequestObject,
) (v1.AdminPublishPromptResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	name := prompts.Name(request.Params.Name)

	prevVersion := 0
	if cur, getErr := h.usecase.Get(ctx, name, adminID); getErr == nil {
		prevVersion = cur.Current.Version
	}
	v, err := h.usecase.Publish(ctx, name, adminID)
	if err != nil {
		return nil, err
	}
	h.audit.Record(ctx, audit.Entry{
		AdminID:     adminID,
		Target:      audit.TargetPrompt,
		TargetID:    string(name),
		TargetLabel: string(name),
		Verb:        audit.VerbPublish,
		Changes: []audit.Change{
			{Field: "version", From: fmt.Sprintf("v%d", prevVersion), To: fmt.Sprintf("v%d", v.Version)},
		},
	})
	return v1.AdminPublishPrompt200JSONResponse(apimodels.PromptVersionFromUsecase(*v)), nil
}

// AdminRollbackPrompt реализует POST /admin/prompts/rollback?name=<name>&version=<n>.
func (h PromptsHandler) AdminRollbackPrompt(
	ctx context.Context,
	request v1.AdminRollbackPromptRequestObject,
) (v1.AdminRollbackPromptResponseObject, error) {
	adminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	name := prompts.Name(request.Params.Name)

	prevVersion := 0
	if cur, getErr := h.usecase.Get(ctx, name, adminID); getErr == nil {
		prevVersion = cur.Current.Version
	}
	v, err := h.usecase.Rollback(ctx, name, request.Params.Version, adminID)
	if err != nil {
		return nil, err
	}
	h.audit.Record(ctx, audit.Entry{
		AdminID:     adminID,
		Target:      audit.TargetPrompt,
		TargetID:    string(name),
		TargetLabel: string(name),
		Verb:        audit.VerbRollback,
		Changes: []audit.Change{
			{Field: "version", From: fmt.Sprintf("v%d", prevVersion), To: fmt.Sprintf("v%d", v.Version)},
		},
	})
	return v1.AdminRollbackPrompt200JSONResponse(apimodels.PromptVersionFromUsecase(*v)), nil
}
