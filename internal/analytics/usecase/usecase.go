// Package usecase реализует analytics.Usecase.
package usecase

import (
	"context"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/analytics"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

const (
	topDishesLimit       = 5
	topRecommendedLimit  = 10
)

type usecaseImpl struct {
	repo analytics.Repository
}

// New создаёт analytics.Usecase.
func New(repo analytics.Repository) analytics.Usecase {
	return &usecaseImpl{repo: repo}
}

// GetDashboard собирает операционный снимок: заказы / выручка / средний чек /
// разбивка по статусам / топ блюд по продажам.
func (u *usecaseImpl) GetDashboard(
	ctx context.Context,
	period analytics.Period,
) (*usecasemodels.AdminDashboard, error) {
	from, to, err := period.Resolve()
	if err != nil {
		return nil, err
	}
	orders, revenue, err := u.repo.CountOrders(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("count orders: %w", err)
	}
	statusMap, err := u.repo.OrdersByStatus(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("orders by status: %w", err)
	}
	topDishes, err := u.repo.TopDishesByOrders(ctx, from, to, topDishesLimit)
	if err != nil {
		return nil, fmt.Errorf("top dishes: %w", err)
	}
	cartBySource, err := u.repo.CartAdditionsBySource(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("cart additions by source: %w", err)
	}

	statusList := make([]usecasemodels.OrdersByStatusEntry, 0, len(statusMap))
	for status, count := range statusMap {
		statusList = append(statusList, usecasemodels.OrdersByStatusEntry{Status: status, Count: count})
	}

	avgCheck := int64(0)
	if orders > 0 {
		avgCheck = revenue / int64(orders)
	}

	chatAdds := cartBySource["chat"]
	menuAdds := cartBySource["menu"]
	totalAdds := 0
	for _, n := range cartBySource {
		totalAdds += n
	}

	return &usecasemodels.AdminDashboard{
		Period:                string(period),
		Orders:                orders,
		RevenueMinor:          revenue,
		AverageCheckMinor:     avgCheck,
		OrdersByStatus:        statusList,
		TopDishes:             toEntries(topDishes),
		CartAdditionsTotal:    totalAdds,
		CartAdditionsFromChat: chatAdds,
		CartAdditionsFromMenu: menuAdds,
	}, nil
}

// GetAnalytics собирает поведенческий снимок: среднее заказанных / в корзину /
// топ рекомендованных блюд.
func (u *usecaseImpl) GetAnalytics(
	ctx context.Context,
	period analytics.Period,
) (*usecasemodels.AdminAnalytics, error) {
	from, to, err := period.Resolve()
	if err != nil {
		return nil, err
	}
	msgs, err := u.repo.CountAssistantMessagesWithRecommendations(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("count assistant messages: %w", err)
	}
	avgOrdered, err := u.repo.AvgRecommendedOrdered(ctx, from, to, analytics.MatchWindow)
	if err != nil {
		return nil, fmt.Errorf("avg ordered: %w", err)
	}
	avgCart, err := u.repo.AvgRecommendedAddedToCart(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("avg added to cart: %w", err)
	}
	topRec, err := u.repo.TopRecommendedDishes(ctx, from, to, topRecommendedLimit)
	if err != nil {
		return nil, fmt.Errorf("top recommended: %w", err)
	}

	return &usecasemodels.AdminAnalytics{
		Period:                   string(period),
		AssistantMessages:        msgs,
		AvgOrderedPerMessage:     avgOrdered,
		AvgAddedToCartPerMessage: avgCart,
		TopRecommendedDishes:     toEntries(topRec),
	}, nil
}

func toEntries(in []analytics.TopDish) []usecasemodels.TopDishEntry {
	out := make([]usecasemodels.TopDishEntry, len(in))
	for i, t := range in {
		out[i] = usecasemodels.TopDishEntry{DishID: t.DishID, DishName: t.DishName, Value: t.Value}
	}
	return out
}
