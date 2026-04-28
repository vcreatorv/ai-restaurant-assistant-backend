package app

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
)

// Unimplemented заглушки для нереализованных endpoint'ов
type Unimplemented struct{}

func (Unimplemented) AdminGetAnalyticsOverview(_ context.Context, _ v1.AdminGetAnalyticsOverviewRequestObject) (v1.AdminGetAnalyticsOverviewResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AdminListAnalyticsQueries(_ context.Context, _ v1.AdminListAnalyticsQueriesRequestObject) (v1.AdminListAnalyticsQueriesResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AdminListOrders(_ context.Context, _ v1.AdminListOrdersRequestObject) (v1.AdminListOrdersResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AdminGetOrder(_ context.Context, _ v1.AdminGetOrderRequestObject) (v1.AdminGetOrderResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AdminUpdateOrderStatus(_ context.Context, _ v1.AdminUpdateOrderStatusRequestObject) (v1.AdminUpdateOrderStatusResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) ClearCart(_ context.Context, _ v1.ClearCartRequestObject) (v1.ClearCartResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) GetCart(_ context.Context, _ v1.GetCartRequestObject) (v1.GetCartResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AddCartItem(_ context.Context, _ v1.AddCartItemRequestObject) (v1.AddCartItemResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) RemoveCartItem(_ context.Context, _ v1.RemoveCartItemRequestObject) (v1.RemoveCartItemResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) UpdateCartItem(_ context.Context, _ v1.UpdateCartItemRequestObject) (v1.UpdateCartItemResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) ListChats(_ context.Context, _ v1.ListChatsRequestObject) (v1.ListChatsResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) CreateChat(_ context.Context, _ v1.CreateChatRequestObject) (v1.CreateChatResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) DeleteChat(_ context.Context, _ v1.DeleteChatRequestObject) (v1.DeleteChatResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) GetChat(_ context.Context, _ v1.GetChatRequestObject) (v1.GetChatResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) SendMessage(_ context.Context, _ v1.SendMessageRequestObject) (v1.SendMessageResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) ListOrders(_ context.Context, _ v1.ListOrdersRequestObject) (v1.ListOrdersResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) CreateOrder(_ context.Context, _ v1.CreateOrderRequestObject) (v1.CreateOrderResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) GetOrder(_ context.Context, _ v1.GetOrderRequestObject) (v1.GetOrderResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}
