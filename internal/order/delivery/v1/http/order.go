package http

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
)

// CreateOrder реализует POST /orders
func (h OrderHandler) CreateOrder(
	ctx context.Context,
	request v1.CreateOrderRequestObject,
) (v1.CreateOrderResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	o, err := h.usecase.Create(ctx, userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentType(request.Body.FulfillmentType),
		PaymentMethod:   usecasemodels.PaymentMethod(request.Body.PaymentMethod),
		DeliveryAddress: request.Body.DeliveryAddress,
		DeliveryNotes:   request.Body.DeliveryNotes,
		Notes:           request.Body.Notes,
	})
	if err != nil {
		return nil, err
	}
	return v1.CreateOrder201JSONResponse(apimodels.OrderFromUsecase(o)), nil
}

// ListOrders реализует GET /orders — заказы текущего пользователя
func (h OrderHandler) ListOrders(
	ctx context.Context,
	request v1.ListOrdersRequestObject,
) (v1.ListOrdersResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := h.resolveListPaging(request.Params.Limit, request.Params.Offset)
	filter := usecasemodels.OrderListFilter{
		Limit:  limit,
		Offset: offset,
	}
	if request.Params.Status != nil {
		s := usecasemodels.OrderStatus(*request.Params.Status)
		filter.Status = &s
	}
	items, total, err := h.usecase.List(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	return v1.ListOrders200JSONResponse(apimodels.OrderListFromUsecase(items, total, limit, offset)), nil
}

// GetOrder реализует GET /orders/{id}
func (h OrderHandler) GetOrder(
	ctx context.Context,
	request v1.GetOrderRequestObject,
) (v1.GetOrderResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.usecase.Get(ctx, userID, request.Id)
	if err != nil {
		return nil, err
	}
	return v1.GetOrder200JSONResponse(apimodels.OrderFromUsecase(o)), nil
}
