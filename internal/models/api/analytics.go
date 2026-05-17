package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// AdminDashboardFromUsecase маппит доменный снимок dashboard в API.
func AdminDashboardFromUsecase(d usecasemodels.AdminDashboard) AdminDashboard {
	statuses := make([]OrdersByStatusEntry, len(d.OrdersByStatus))
	for i, s := range d.OrdersByStatus {
		statuses[i] = OrdersByStatusEntry{
			Status: OrderStatus(s.Status),
			Count:  s.Count,
		}
	}
	top := make([]TopDishEntry, len(d.TopDishes))
	for i, t := range d.TopDishes {
		top[i] = topDishEntryFromUsecase(t)
	}
	return AdminDashboard{
		Period:                AdminDashboardPeriod(d.Period),
		Orders:                d.Orders,
		RevenueMinor:          int(d.RevenueMinor),
		AverageCheckMinor:     int(d.AverageCheckMinor),
		OrdersByStatus:        statuses,
		TopDishes:             top,
		CartAdditionsTotal:    d.CartAdditionsTotal,
		CartAdditionsFromChat: d.CartAdditionsFromChat,
		CartAdditionsFromMenu: d.CartAdditionsFromMenu,
	}
}

// AdminAnalyticsFromUsecase маппит доменный снимок analytics в API.
func AdminAnalyticsFromUsecase(a usecasemodels.AdminAnalytics) AdminAnalytics {
	top := make([]TopDishEntry, len(a.TopRecommendedDishes))
	for i, t := range a.TopRecommendedDishes {
		top[i] = topDishEntryFromUsecase(t)
	}
	return AdminAnalytics{
		Period:                    AdminAnalyticsPeriod(a.Period),
		AssistantMessages:         a.AssistantMessages,
		AvgOrderedPerMessage:      float32(a.AvgOrderedPerMessage),
		AvgAddedToCartPerMessage:  float32(a.AvgAddedToCartPerMessage),
		TopRecommendedDishes:      top,
	}
}

func topDishEntryFromUsecase(t usecasemodels.TopDishEntry) TopDishEntry {
	return TopDishEntry{
		DishId:   t.DishID,
		DishName: t.DishName,
		Value:    t.Value,
	}
}
