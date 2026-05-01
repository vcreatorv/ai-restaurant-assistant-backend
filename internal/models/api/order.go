package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// OrderFromUsecase маппит usecasemodels.Order в api-DTO Order
func OrderFromUsecase(o *usecasemodels.Order) Order {
	items := make([]OrderItem, 0, len(o.Items))
	for i := range o.Items {
		items = append(items, OrderItemFromUsecase(&o.Items[i]))
	}
	out := Order{
		Id:                o.ID,
		UserId:            o.UserID,
		Status:            OrderStatus(o.Status),
		FulfillmentType:   FulfillmentType(o.FulfillmentType),
		PaymentMethod:     PaymentMethod(o.PaymentMethod),
		TotalMinor:        o.TotalMinor,
		Currency:          o.Currency,
		CustomerFirstName: o.CustomerFirstName,
		CustomerLastName:  o.CustomerLastName,
		CustomerPhone:     o.CustomerPhone,
		DeliveryAddress:   o.DeliveryAddress,
		DeliveryNotes:     o.DeliveryNotes,
		Notes:             o.Notes,
		Items:             items,
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}
	if o.CustomerEmail != nil {
		em := openapi_types.Email(*o.CustomerEmail)
		out.CustomerEmail = &em
	}
	return out
}

// OrderItemFromUsecase маппит usecasemodels.OrderItem в api-DTO OrderItem
func OrderItemFromUsecase(it *usecasemodels.OrderItem) OrderItem {
	return OrderItem{
		DishId:         it.DishID,
		DishName:       it.DishName,
		DishPriceMinor: it.DishPriceMinor,
		Quantity:       it.Quantity,
		LineTotalMinor: it.LineTotalMinor,
		SortOrder:      it.SortOrder,
	}
}

// OrderListFromUsecase собирает api-DTO списка заказов
func OrderListFromUsecase(items []usecasemodels.Order, total, limit, offset int) OrderList {
	out := OrderList{
		Items:  make([]Order, 0, len(items)),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
	for i := range items {
		out.Items = append(out.Items, OrderFromUsecase(&items[i]))
	}
	return out
}
