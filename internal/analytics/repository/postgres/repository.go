// Package postgres реализует analytics.Repository поверх PostgreSQL (pgx/v5).
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/analytics"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository PostgreSQL-репозиторий для аналитики.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Статусы заказов, которые мы НЕ учитываем в выручке и топах продаж.
// 'cancelled' — единственный, остальные («accepted», «cooking», «ready», «in_delivery», «closed»)
// представляют реальную или потенциальную выручку.
const excludeStatusCancelled = `'cancelled'`

const countOrdersQuery = `
	SELECT
		COUNT(*) FILTER (WHERE o.status NOT IN (` + excludeStatusCancelled + `))::int,
		COALESCE(SUM(o.total_minor) FILTER (WHERE o.status NOT IN (` + excludeStatusCancelled + `)), 0)::bigint
	FROM orders o
	WHERE o.created_at >= $1 AND o.created_at < $2`

// CountOrders возвращает кол-во заказов и выручку.
func (r *Repository) CountOrders(ctx context.Context, from, to time.Time) (int, int64, error) {
	var n int
	var rev int64
	if err := r.pool.QueryRow(ctx, countOrdersQuery, from, to).Scan(&n, &rev); err != nil {
		return 0, 0, fmt.Errorf("count orders: %w", err)
	}
	return n, rev, nil
}

const ordersByStatusQuery = `
	SELECT status, COUNT(*)::int
	FROM orders
	WHERE created_at >= $1 AND created_at < $2
	GROUP BY status`

// OrdersByStatus возвращает map статус → количество.
func (r *Repository) OrdersByStatus(ctx context.Context, from, to time.Time) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, ordersByStatusQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("orders by status: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const topDishesByOrdersQuery = `
	SELECT oi.dish_id, d.name, COALESCE(SUM(oi.quantity), 0)::int AS sold
	FROM order_items oi
	JOIN orders o ON o.id = oi.order_id
	JOIN dishes d ON d.id = oi.dish_id
	WHERE o.created_at >= $1 AND o.created_at < $2
	  AND o.status NOT IN (` + excludeStatusCancelled + `)
	GROUP BY oi.dish_id, d.name
	ORDER BY sold DESC
	LIMIT $3`

// TopDishesByOrders топ блюд по продажам.
func (r *Repository) TopDishesByOrders(
	ctx context.Context, from, to time.Time, limit int,
) ([]analytics.TopDish, error) {
	return r.queryTopDishes(ctx, topDishesByOrdersQuery, from, to, limit)
}

const topRecommendedDishesQuery = `
	SELECT dish_id, d.name, COUNT(*)::int AS recs
	FROM chat_messages cm,
	     LATERAL unnest(cm.recommended_dish_ids) AS dish_id
	JOIN dishes d ON d.id = dish_id
	WHERE cm.role = 'assistant'
	  AND cm.created_at >= $1 AND cm.created_at < $2
	  AND array_length(cm.recommended_dish_ids, 1) > 0
	GROUP BY dish_id, d.name
	ORDER BY recs DESC
	LIMIT $3`

// TopRecommendedDishes топ по частоте появления в recommended_dish_ids.
func (r *Repository) TopRecommendedDishes(
	ctx context.Context, from, to time.Time, limit int,
) ([]analytics.TopDish, error) {
	return r.queryTopDishes(ctx, topRecommendedDishesQuery, from, to, limit)
}

func (r *Repository) queryTopDishes(
	ctx context.Context, sql string, from, to time.Time, limit int,
) ([]analytics.TopDish, error) {
	rows, err := r.pool.Query(ctx, sql, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("top dishes query: %w", err)
	}
	defer rows.Close()
	var out []analytics.TopDish
	for rows.Next() {
		var td analytics.TopDish
		if err := rows.Scan(&td.DishID, &td.DishName, &td.Value); err != nil {
			return nil, err
		}
		out = append(out, td)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const countAssistantMsgsQuery = `
	SELECT COUNT(*)::int
	FROM chat_messages
	WHERE role = 'assistant'
	  AND created_at >= $1 AND created_at < $2
	  AND array_length(recommended_dish_ids, 1) > 0`

// CountAssistantMessagesWithRecommendations знаменатель для «среднее заказано/в корзину per message».
func (r *Repository) CountAssistantMessagesWithRecommendations(
	ctx context.Context, from, to time.Time,
) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, countAssistantMsgsQuery, from, to).Scan(&n); err != nil {
		return 0, fmt.Errorf("count assistant messages: %w", err)
	}
	return n, nil
}

// avgOrderedQuery: для каждого assistant-сообщения с рекомендациями считаем
// |recommended ∩ заказанные dish_id того же user в окне MatchWindow|, потом усредняем.
//
// Связка по c.user_id, поскольку assistant-msg прикреплено к chat'у, а chat — к user.
const avgOrderedQuery = `
	WITH msg AS (
		SELECT cm.id          AS msg_id,
		       c.user_id      AS user_id,
		       cm.created_at  AS msg_at,
		       cm.recommended_dish_ids AS recs
		FROM chat_messages cm
		JOIN chats c ON c.id = cm.chat_id
		WHERE cm.role = 'assistant'
		  AND cm.created_at >= $1 AND cm.created_at < $2
		  AND array_length(cm.recommended_dish_ids, 1) > 0
	),
	matches AS (
		SELECT m.msg_id, COUNT(DISTINCT oi.dish_id) AS ordered_from_recs
		FROM msg m
		LEFT JOIN orders o
		       ON o.user_id = m.user_id
		      AND o.created_at >  m.msg_at
		      AND o.created_at <= m.msg_at + make_interval(secs => $3)
		      AND o.status NOT IN (` + excludeStatusCancelled + `)
		LEFT JOIN order_items oi
		       ON oi.order_id = o.id
		      AND oi.dish_id = ANY(m.recs)
		GROUP BY m.msg_id
	)
	SELECT COALESCE(AVG(ordered_from_recs), 0)::float8 FROM matches`

// AvgRecommendedOrdered среднее блюд из рекомендаций, реально купленных.
func (r *Repository) AvgRecommendedOrdered(
	ctx context.Context, from, to time.Time, matchWindow time.Duration,
) (float64, error) {
	var avg float64
	seconds := int(matchWindow.Seconds())
	if err := r.pool.QueryRow(ctx, avgOrderedQuery, from, to, seconds).Scan(&avg); err != nil {
		return 0, fmt.Errorf("avg ordered: %w", err)
	}
	return avg, nil
}

// avgAddedToCartQuery: для каждого assistant-сообщения с рекомендациями считаем
// |dish_id ∩ recommended_dish_ids|, где dish_id — из cart_additions с source='chat' и
// message_id = id ассистент-сообщения. Усредняем по сообщениям.
const avgAddedToCartQuery = `
	WITH msg AS (
		SELECT cm.id AS msg_id, cm.recommended_dish_ids AS recs
		FROM chat_messages cm
		WHERE cm.role = 'assistant'
		  AND cm.created_at >= $1 AND cm.created_at < $2
		  AND array_length(cm.recommended_dish_ids, 1) > 0
	),
	per_msg AS (
		SELECT m.msg_id, COUNT(DISTINCT ca.dish_id) FILTER (
			WHERE ca.dish_id = ANY(m.recs)
		) AS added_from_recs
		FROM msg m
		LEFT JOIN cart_additions ca
		       ON ca.message_id = m.msg_id
		      AND ca.source = 'chat'
		GROUP BY m.msg_id
	)
	SELECT COALESCE(AVG(added_from_recs), 0)::float8 FROM per_msg`

// AvgRecommendedAddedToCart среднее блюд из рекомендаций, добавленных в корзину «из чата».
func (r *Repository) AvgRecommendedAddedToCart(
	ctx context.Context, from, to time.Time,
) (float64, error) {
	var avg float64
	if err := r.pool.QueryRow(ctx, avgAddedToCartQuery, from, to).Scan(&avg); err != nil {
		return 0, fmt.Errorf("avg added to cart: %w", err)
	}
	return avg, nil
}

const cartAdditionsBySourceQuery = `
	SELECT source, COUNT(*)::int
	FROM cart_additions
	WHERE created_at >= $1 AND created_at < $2
	GROUP BY source`

// CartAdditionsBySource количество событий добавления в корзину за период, в разбивке по source.
func (r *Repository) CartAdditionsBySource(
	ctx context.Context, from, to time.Time,
) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, cartAdditionsBySourceQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("cart additions by source: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		out[source] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
