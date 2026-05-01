package http

import (
	"context"
	"fmt"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
)

// GetCart реализует GET /cart
func (h CartHandler) GetCart(
	ctx context.Context,
	_ v1.GetCartRequestObject,
) (v1.GetCartResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.usecase.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return v1.GetCart200JSONResponse(apimodels.CartFromUsecase(v)), nil
}

// ClearCart реализует DELETE /cart
func (h CartHandler) ClearCart(
	ctx context.Context,
	_ v1.ClearCartRequestObject,
) (v1.ClearCartResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.Clear(ctx, userID); err != nil {
		return nil, err
	}
	return v1.ClearCart204Response{}, nil
}

// AddCartItem реализует POST /cart/items
func (h CartHandler) AddCartItem(
	ctx context.Context,
	request v1.AddCartItemRequestObject,
) (v1.AddCartItemResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	view, err := h.usecase.AddItem(ctx, userID, usecasemodels.CartItemAdd{
		DishID:   request.Body.DishId,
		Quantity: request.Body.Quantity,
		Note:     request.Body.Note,
	})
	if err != nil {
		return nil, err
	}
	item := findItemView(view, request.Body.DishId)
	if item == nil {
		// Не должно случиться: только что добавили — позиция обязана быть в view.
		return nil, fmt.Errorf("cart: item dish_id=%d not found in view after add", request.Body.DishId)
	}
	return v1.AddCartItem201JSONResponse(apimodels.CartItemFromUsecase(item)), nil
}

// UpdateCartItem реализует PATCH /cart/items/{dish_id}
func (h CartHandler) UpdateCartItem(
	ctx context.Context,
	request v1.UpdateCartItemRequestObject,
) (v1.UpdateCartItemResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	dishID := request.DishId
	view, err := h.usecase.PatchItem(ctx, userID, dishID, usecasemodels.CartItemPatch{
		Quantity:  request.Body.Quantity,
		Note:      request.Body.Note,
		SortOrder: request.Body.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	item := findItemView(view, dishID)
	if item == nil {
		return nil, cart.ErrCartItemNotFound
	}
	return v1.UpdateCartItem200JSONResponse(apimodels.CartItemFromUsecase(item)), nil
}

// RemoveCartItem реализует DELETE /cart/items/{dish_id}
func (h CartHandler) RemoveCartItem(
	ctx context.Context,
	request v1.RemoveCartItemRequestObject,
) (v1.RemoveCartItemResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.RemoveItem(ctx, userID, request.DishId); err != nil {
		return nil, err
	}
	return v1.RemoveCartItem204Response{}, nil
}

// findItemView ищет позицию по dish_id в собранной CartView
func findItemView(v *usecasemodels.CartView, dishID int) *usecasemodels.CartItemView {
	for i := range v.Items {
		if v.Items[i].DishID == dishID {
			return &v.Items[i]
		}
	}
	return nil
}
