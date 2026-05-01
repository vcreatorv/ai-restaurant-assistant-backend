package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// CartFromUsecase маппит CartView в api-DTO Cart
func CartFromUsecase(v *usecasemodels.CartView) Cart {
	items := make([]CartItem, 0, len(v.Items))
	for i := range v.Items {
		items = append(items, CartItemFromUsecase(&v.Items[i]))
	}
	warnings := make([]CartWarning, 0, len(v.Warnings))
	for _, w := range v.Warnings {
		warnings = append(warnings, CartWarning{Code: w.Code, DishId: w.DishID})
	}
	return Cart{
		Items:      items,
		TotalMinor: v.TotalMinor,
		Currency:   v.Currency,
		Warnings:   warnings,
	}
}

// CartItemFromUsecase маппит одну CartItemView в api-DTO CartItem
func CartItemFromUsecase(v *usecasemodels.CartItemView) CartItem {
	addedAt := v.AddedAt
	return CartItem{
		DishId:         v.DishID,
		Name:           v.Name,
		PriceMinor:     v.PriceMinor,
		Quantity:       v.Quantity,
		Note:           v.Note,
		SortOrder:      v.SortOrder,
		AddedAt:        &addedAt,
		Available:      v.Available,
		LineTotalMinor: v.LineTotalMinor,
	}
}
