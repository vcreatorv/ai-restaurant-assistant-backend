package http

import (
	"context"
	"fmt"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"

	"github.com/google/uuid"
)

// AdminListOrders реализует GET /admin/orders — все заказы с фильтрами
func (h OrderHandler) AdminListOrders(
	ctx context.Context,
	request v1.AdminListOrdersRequestObject,
) (v1.AdminListOrdersResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit, offset := h.resolveListPaging(request.Params.Limit, request.Params.Offset)
	filter := usecasemodels.OrderListFilter{
		From:   request.Params.From,
		To:     request.Params.To,
		Limit:  limit,
		Offset: offset,
	}
	if request.Params.Status != nil {
		s := usecasemodels.OrderStatus(*request.Params.Status)
		filter.Status = &s
	}
	var userIDPtr *uuid.UUID
	if request.Params.UserId != nil {
		uid := *request.Params.UserId
		userIDPtr = &uid
	}
	items, total, err := h.usecase.AdminList(ctx, filter, userIDPtr)
	if err != nil {
		return nil, err
	}
	return v1.AdminListOrders200JSONResponse(apimodels.OrderListFromUsecase(items, total, limit, offset)), nil
}

// AdminGetOrder реализует GET /admin/orders/{id}
func (h OrderHandler) AdminGetOrder(
	ctx context.Context,
	request v1.AdminGetOrderRequestObject,
) (v1.AdminGetOrderResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	o, err := h.usecase.AdminGet(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return v1.AdminGetOrder200JSONResponse(apimodels.OrderFromUsecase(o)), nil
}

// AdminUpdateOrderStatus реализует PATCH /admin/orders/{id}/status.
// После успешной смены статуса пишет запись в admin_actions
// (best-effort — фейл записи не валит ответ клиенту).
func (h OrderHandler) AdminUpdateOrderStatus(
	ctx context.Context,
	request v1.AdminUpdateOrderStatusRequestObject,
) (v1.AdminUpdateOrderStatusResponseObject, error) {
	adminID, err := h.requireAdminID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}

	// Читаем текущее состояние, чтобы зафиксировать from-статус в логе.
	prev, err := h.usecase.AdminGet(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	prevStatus := string(prev.Status)
	newStatus := string(request.Body.Status)

	o, err := h.usecase.AdminUpdateStatus(ctx, request.Id, usecasemodels.OrderStatus(request.Body.Status))
	if err != nil {
		return nil, err
	}

	h.audit.Record(ctx, audit.Entry{
		AdminID:     adminID,
		Target:      audit.TargetOrder,
		TargetID:    request.Id.String(),
		TargetLabel: fmt.Sprintf("#%s", shortOrderID(request.Id)),
		Verb:        audit.VerbStatusChange,
		Changes:     []audit.Change{{Field: "status", From: prevStatus, To: newStatus}},
	})

	return v1.AdminUpdateOrderStatus200JSONResponse(apimodels.OrderFromUsecase(o)), nil
}

// shortOrderID берёт первые 8 символов uuid — этого достаточно для отображения в админке
// («Заказ #4f3a8b1c»). Полный uuid у нас в TargetID.
func shortOrderID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}
