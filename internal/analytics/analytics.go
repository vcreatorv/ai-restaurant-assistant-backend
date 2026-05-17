// Package analytics — операционные и поведенческие метрики для админ-панели.
//
// Содержит две независимые подвыборки:
//
//   - Dashboard — операционная сводка по заказам (count, revenue, средний чек,
//     по статусам, топ блюд по продажам);
//   - Analytics — поведение ассистента (среднее реализованных рекомендаций,
//     среднее в корзину из чата, топ рекомендованных блюд).
//
// Период задаётся одним из enum'ов Period; репозиторий резолвит его в from/to.
package analytics

import (
	"context"
	"errors"
	"time"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// ErrInvalidPeriod недопустимое значение периода.
var ErrInvalidPeriod = errors.New("invalid period")

// Period диапазон агрегации.
type Period string

const (
	// PeriodToday — текущие сутки (00:00 локального TZ → now).
	PeriodToday Period = "today"
	// PeriodWeek — последние 7 суток.
	PeriodWeek Period = "week"
	// PeriodMonth — последние 30 суток.
	PeriodMonth Period = "month"
)

// Valid возвращает true, если значение в whitelist'е.
func (p Period) Valid() bool {
	switch p {
	case PeriodToday, PeriodWeek, PeriodMonth:
		return true
	default:
		return false
	}
}

// Resolve возвращает [from, now) для агрегатов SQL.
//
// Использует UTC. На MVP отдельный таймзоны не нужен — все агрегаты идут
// по неделям/месяцам, погрешность <24h несущественна.
func (p Period) Resolve() (from, to time.Time, err error) {
	now := time.Now().UTC()
	switch p {
	case PeriodToday:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), now, nil
	case PeriodWeek:
		return now.AddDate(0, 0, -7), now, nil
	case PeriodMonth:
		return now.AddDate(0, 0, -30), now, nil
	default:
		return time.Time{}, time.Time{}, ErrInvalidPeriod
	}
}

// Usecase сценарии админ-аналитики.
type Usecase interface {
	GetDashboard(ctx context.Context, period Period) (*usecasemodels.AdminDashboard, error)
	GetAnalytics(ctx context.Context, period Period) (*usecasemodels.AdminAnalytics, error)
}

// Repository хранилище для агрегатов.
type Repository interface {
	// CountOrders возвращает (количество заказов, сумма выручки в копейках) за период.
	// Учитываются только статусы, влияющие на выручку (не cancelled).
	CountOrders(ctx context.Context, from, to time.Time) (count int, revenueMinor int64, err error)
	// OrdersByStatus возвращает разбивку count'ов по статусам.
	OrdersByStatus(ctx context.Context, from, to time.Time) (map[string]int, error)
	// TopDishesByOrders топ блюд по проданным позициям (без cancelled).
	TopDishesByOrders(ctx context.Context, from, to time.Time, limit int) ([]TopDish, error)
	// TopRecommendedDishes топ блюд по количеству появлений в recommended_dish_ids.
	TopRecommendedDishes(ctx context.Context, from, to time.Time, limit int) ([]TopDish, error)
	// CountAssistantMessagesWithRecommendations число assistant-ответов с непустым массивом рекомендаций.
	CountAssistantMessagesWithRecommendations(ctx context.Context, from, to time.Time) (int, error)
	// AvgRecommendedOrdered среднее количество рекомендованных блюд, заказанных в окне MatchWindow после ответа.
	AvgRecommendedOrdered(ctx context.Context, from, to time.Time, matchWindow time.Duration) (float64, error)
	// AvgRecommendedAddedToCart среднее количество рекомендованных блюд, добавленных в корзину
	// с source='chat' и message_id совпадающим с assistant-сообщением.
	AvgRecommendedAddedToCart(ctx context.Context, from, to time.Time) (float64, error)
	// CartAdditionsBySource — количество «+» в корзину за период с разбивкой по source.
	// В map'е ключи — те же значения source, что в БД (chat/menu/cart/other); отсутствующие
	// ключи означают «0 за период», обрабатывать в usecase.
	CartAdditionsBySource(ctx context.Context, from, to time.Time) (map[string]int, error)
}

// TopDish одна строка топа.
type TopDish struct {
	DishID   int
	DishName string
	Value    int
}

// MatchWindow окно, в котором мы считаем, что заказ был сделан «после рекомендации».
// 60 минут — компромисс между мгновенным заказом и долгим обсуждением в чате.
const MatchWindow = 60 * time.Minute
