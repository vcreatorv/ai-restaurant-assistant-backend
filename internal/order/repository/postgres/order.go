package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/example/ai-restaurant-assistant-backend/internal/order"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const orderColumns = `
	id, user_id, status, fulfillment_type, payment_method,
	total_minor, currency,
	customer_first_name, customer_last_name, customer_phone, customer_email,
	delivery_address, delivery_notes, notes,
	created_at, updated_at`

const orderItemColumns = `order_id, dish_id, name_snapshot, price_minor_snapshot, quantity, sort_order`

const (
	insertOrderQuery = `
		INSERT INTO orders (
			id, user_id, status, fulfillment_type, payment_method,
			total_minor, currency,
			customer_first_name, customer_last_name, customer_phone, customer_email,
			delivery_address, delivery_notes, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9, $10, $11,
			$12, $13, $14
		)
		RETURNING created_at, updated_at`

	insertOrderItemQuery = `
		INSERT INTO order_items (order_id, dish_id, name_snapshot, price_minor_snapshot, quantity, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)`

	findOrderByIDQuery = `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE id = $1`

	listOrderItemsByOrderQuery = `
		SELECT ` + orderItemColumns + `
		FROM order_items
		WHERE order_id = $1
		ORDER BY sort_order, dish_id`

	listOrderItemsByOrderIDsQuery = `
		SELECT ` + orderItemColumns + `
		FROM order_items
		WHERE order_id = ANY($1::uuid[])
		ORDER BY order_id, sort_order, dish_id`

	updateOrderStatusQuery = `
		UPDATE orders
		SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING ` + orderColumns
)

// CreateOrder вставляет заказ + позиции в одной транзакции
func (r *Repository) CreateOrder(
	ctx context.Context,
	o *repositorymodels.Order,
	items []repositorymodels.OrderItem,
) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, insertOrderQuery,
		o.ID, o.UserID, o.Status, o.FulfillmentType, o.PaymentMethod,
		o.TotalMinor, o.Currency,
		o.CustomerFirstName, o.CustomerLastName, o.CustomerPhone, o.CustomerEmail,
		o.DeliveryAddress, o.DeliveryNotes, o.Notes,
	).Scan(&o.CreatedAt, &o.UpdatedAt); err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for i := range items {
		it := &items[i]
		if _, ierr := tx.Exec(ctx, insertOrderItemQuery,
			o.ID, it.DishID, it.NameSnapshot, it.PriceMinorSnapshot, it.Quantity, it.SortOrder,
		); ierr != nil {
			return fmt.Errorf("insert order item dish_id=%d: %w", it.DishID, ierr)
		}
	}
	return tx.Commit(ctx)
}

// FindOrderByID возвращает заказ + его позиции; ErrOrderNotFound если нет
func (r *Repository) FindOrderByID(
	ctx context.Context,
	orderID uuid.UUID,
) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
	row := r.pool.QueryRow(ctx, findOrderByIDQuery, orderID)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, order.ErrOrderNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("find order: %w", err)
	}

	items, err := r.listOrderItems(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return o, items, nil
}

// ListOrders возвращает заказы по фильтру + total
func (r *Repository) ListOrders(
	ctx context.Context,
	f repositorymodels.OrderFilter,
) ([]repositorymodels.Order, int, error) {
	whereParts := []string{"1=1"}
	args := []any{}
	idx := 1
	if f.UserID != nil {
		whereParts = append(whereParts, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.Status != nil {
		whereParts = append(whereParts, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.From != nil {
		whereParts = append(whereParts, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		whereParts = append(whereParts, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	whereSQL := strings.Join(whereParts, " AND ")

	countQuery := "SELECT COUNT(*) FROM orders WHERE " + whereSQL
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	listQuery := "SELECT " + orderColumns + " FROM orders WHERE " + whereSQL +
		fmt.Sprintf(" ORDER BY created_at DESC, id LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	out := make([]repositorymodels.Order, 0, f.Limit)
	for rows.Next() {
		o, oerr := scanOrder(rows)
		if oerr != nil {
			return nil, 0, oerr
		}
		out = append(out, *o)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, 0, fmt.Errorf("rows orders: %w", rerr)
	}
	return out, total, nil
}

// LoadOrderItems batch-загрузка позиций для списка заказов
func (r *Repository) LoadOrderItems(
	ctx context.Context,
	orderIDs []uuid.UUID,
) (map[uuid.UUID][]repositorymodels.OrderItem, error) {
	out := make(map[uuid.UUID][]repositorymodels.OrderItem, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, listOrderItemsByOrderIDsQuery, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("query order_items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		it, ierr := scanOrderItem(rows)
		if ierr != nil {
			return nil, ierr
		}
		out[it.OrderID] = append(out[it.OrderID], *it)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("rows order_items: %w", rerr)
	}
	return out, nil
}

// UpdateOrderStatus обновляет status + updated_at; ErrOrderNotFound если нет
func (r *Repository) UpdateOrderStatus(
	ctx context.Context,
	orderID uuid.UUID,
	newStatus string,
) (*repositorymodels.Order, error) {
	row := r.pool.QueryRow(ctx, updateOrderStatusQuery, orderID, newStatus)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, order.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}
	return o, nil
}

// listOrderItems возвращает позиции одного заказа
func (r *Repository) listOrderItems(
	ctx context.Context,
	orderID uuid.UUID,
) ([]repositorymodels.OrderItem, error) {
	rows, err := r.pool.Query(ctx, listOrderItemsByOrderQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("query order_items: %w", err)
	}
	defer rows.Close()
	out := make([]repositorymodels.OrderItem, 0)
	for rows.Next() {
		it, ierr := scanOrderItem(rows)
		if ierr != nil {
			return nil, ierr
		}
		out = append(out, *it)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("rows order_items: %w", rerr)
	}
	return out, nil
}

func scanOrder(row pgx.Row) (*repositorymodels.Order, error) {
	var o repositorymodels.Order
	if err := row.Scan(
		&o.ID, &o.UserID, &o.Status, &o.FulfillmentType, &o.PaymentMethod,
		&o.TotalMinor, &o.Currency,
		&o.CustomerFirstName, &o.CustomerLastName, &o.CustomerPhone, &o.CustomerEmail,
		&o.DeliveryAddress, &o.DeliveryNotes, &o.Notes,
		&o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &o, nil
}

func scanOrderItem(row pgx.Row) (*repositorymodels.OrderItem, error) {
	var it repositorymodels.OrderItem
	if err := row.Scan(
		&it.OrderID, &it.DishID, &it.NameSnapshot, &it.PriceMinorSnapshot, &it.Quantity, &it.SortOrder,
	); err != nil {
		return nil, err
	}
	return &it, nil
}
