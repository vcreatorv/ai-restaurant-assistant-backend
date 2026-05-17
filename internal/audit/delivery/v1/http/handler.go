// Package http — HTTP-делирий аудит-лога: GET /admin/actions и /admin/orders/{id}/actions.
package http

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	auditusecase "github.com/example/ai-restaurant-assistant-backend/internal/audit/usecase"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"
)

// AuditHandler HTTP-делирий чтения аудит-лога.
type AuditHandler struct {
	reader auditusecase.Reader
	users  user.Usecase
}

// New создаёт AuditHandler.
func New(reader auditusecase.Reader, users user.Usecase) AuditHandler {
	return AuditHandler{reader: reader, users: users}
}

// requireAdmin проверяет, что текущая сессия админ. Возвращает id админа.
func (h AuditHandler) requireAdmin(ctx context.Context) (uuid.UUID, error) {
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

// AdminListActions реализует GET /admin/actions.
// Поддерживает фильтр admin_id=<uuid> либо admin_id=me.
func (h AuditHandler) AdminListActions(
	ctx context.Context,
	request v1.AdminListActionsRequestObject,
) (v1.AdminListActionsResponseObject, error) {
	currentAdminID, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	f := audit.Filter{}
	if p := request.Params.AdminId; p != nil && *p != "" {
		switch *p {
		case "me":
			id := currentAdminID
			f.AdminID = &id
		default:
			parsed, err := uuid.Parse(*p)
			if err != nil {
				return nil, apperrors.ErrBadRequest
			}
			f.AdminID = &parsed
		}
	}
	if p := request.Params.Target; p != nil {
		t := audit.Target(*p)
		f.Target = &t
	}
	if p := request.Params.From; p != nil {
		from := *p
		f.From = &from
	}
	if p := request.Params.To; p != nil {
		to := *p
		f.To = &to
	}
	if p := request.Params.Limit; p != nil {
		f.Limit = *p
	}
	if p := request.Params.Offset; p != nil {
		f.Offset = *p
	}

	items, total, err := h.reader.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return v1.AdminListActions200JSONResponse(
		apimodels.AdminActionListFromUsecase(items, total, f.Limit, f.Offset),
	), nil
}

// AdminListOrderActions реализует GET /admin/orders/{id}/actions.
func (h AuditHandler) AdminListOrderActions(
	ctx context.Context,
	request v1.AdminListOrderActionsRequestObject,
) (v1.AdminListOrderActionsResponseObject, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	limit, offset := 0, 0
	if p := request.Params.Limit; p != nil {
		limit = *p
	}
	if p := request.Params.Offset; p != nil {
		offset = *p
	}

	items, total, err := h.reader.ListByOrder(ctx, uuid.UUID(request.Id), limit, offset)
	if err != nil {
		return nil, err
	}
	return v1.AdminListOrderActions200JSONResponse(
		apimodels.AdminActionListFromUsecase(items, total, limit, offset),
	), nil
}
