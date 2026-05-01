package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const cartColumns = `id, user_id, created_at, updated_at`

const cartItemColumns = `cart_id, dish_id, quantity, note, sort_order, added_at`

const (
	findCartByUserQuery = `
		SELECT ` + cartColumns + `
		FROM carts
		WHERE user_id = $1`

	insertCartQuery = `
		INSERT INTO carts (id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING ` + cartColumns

	listCartItemsQuery = `
		SELECT ` + cartItemColumns + `
		FROM cart_items
		WHERE cart_id = $1
		ORDER BY sort_order, added_at`

	findCartItemQuery = `
		SELECT ` + cartItemColumns + `
		FROM cart_items
		WHERE cart_id = $1 AND dish_id = $2`

	// upsertCartItemQuery: insert или, при конфликте, прибавить delta к quantity.
	// Возвращает финальную строку. note обновляется только если переданный != NULL
	// (передадим NULL когда не хотим менять заметку при суммировании).
	upsertCartItemQuery = `
		INSERT INTO cart_items (cart_id, dish_id, quantity, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cart_id, dish_id) DO UPDATE
		SET quantity = cart_items.quantity + EXCLUDED.quantity,
		    note = COALESCE(EXCLUDED.note, cart_items.note)
		RETURNING ` + cartItemColumns

	// setCartItemQuantityQuery — жёсткое выставление quantity (PATCH).
	setCartItemQuantityQuery = `
		UPDATE cart_items
		SET quantity = $3
		WHERE cart_id = $1 AND dish_id = $2
		RETURNING ` + cartItemColumns

	// patchCartItemFieldsQuery — обновляем только note и/или sort_order.
	// COALESCE($3, note): если $3 NULL — оставляем старое; чтобы стереть заметку,
	// передаём пустую строку (БД хранит как ''), а не NULL.
	patchCartItemFieldsQuery = `
		UPDATE cart_items
		SET note = COALESCE($3, note),
		    sort_order = COALESCE($4, sort_order)
		WHERE cart_id = $1 AND dish_id = $2
		RETURNING ` + cartItemColumns

	deleteCartItemQuery = `DELETE FROM cart_items WHERE cart_id = $1 AND dish_id = $2`

	deleteAllCartItemsQuery = `DELETE FROM cart_items WHERE cart_id = $1`

	touchCartQuery = `UPDATE carts SET updated_at = now() WHERE id = $1`
)

// FindOrCreateCart возвращает корзину пользователя; создаёт пустую если её нет
func (r *Repository) FindOrCreateCart(
	ctx context.Context,
	userID uuid.UUID,
) (*repositorymodels.Cart, error) {
	row := r.pool.QueryRow(ctx, findCartByUserQuery, userID)
	c, err := scanCart(row)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find cart: %w", err)
	}
	row = r.pool.QueryRow(ctx, insertCartQuery, uuid.New(), userID)
	c, err = scanCart(row)
	if err != nil {
		return nil, fmt.Errorf("insert cart: %w", err)
	}
	return c, nil
}

// ListItems возвращает позиции корзины (sort_order, added_at)
func (r *Repository) ListItems(
	ctx context.Context,
	cartID uuid.UUID,
) ([]repositorymodels.CartItem, error) {
	rows, err := r.pool.Query(ctx, listCartItemsQuery, cartID)
	if err != nil {
		return nil, fmt.Errorf("query cart items: %w", err)
	}
	defer rows.Close()

	out := make([]repositorymodels.CartItem, 0)
	for rows.Next() {
		it, ierr := scanCartItem(rows)
		if ierr != nil {
			return nil, ierr
		}
		out = append(out, *it)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("rows cart items: %w", rerr)
	}
	return out, nil
}

// UpsertItem вставляет позицию или прибавляет quantityDelta к существующей.
// Транзакция: insert/update cart_items + touch carts.updated_at.
func (r *Repository) UpsertItem(
	ctx context.Context,
	cartID uuid.UUID,
	dishID, quantityDelta int,
	note *string,
) (*repositorymodels.CartItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, upsertCartItemQuery, cartID, dishID, quantityDelta, note)
	it, err := scanCartItem(row)
	if err != nil {
		return nil, fmt.Errorf("upsert cart item: %w", err)
	}
	if _, terr := tx.Exec(ctx, touchCartQuery, cartID); terr != nil {
		return nil, fmt.Errorf("touch cart: %w", terr)
	}
	if cerr := tx.Commit(ctx); cerr != nil {
		return nil, fmt.Errorf("commit tx: %w", cerr)
	}
	return it, nil
}

// SetItemQuantity жёстко выставляет quantity у существующей позиции
func (r *Repository) SetItemQuantity(
	ctx context.Context,
	cartID uuid.UUID,
	dishID, quantity int,
) (*repositorymodels.CartItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, setCartItemQuantityQuery, cartID, dishID, quantity)
	it, err := scanCartItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cart.ErrCartItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set cart item quantity: %w", err)
	}
	if _, terr := tx.Exec(ctx, touchCartQuery, cartID); terr != nil {
		return nil, fmt.Errorf("touch cart: %w", terr)
	}
	if cerr := tx.Commit(ctx); cerr != nil {
		return nil, fmt.Errorf("commit tx: %w", cerr)
	}
	return it, nil
}

// PatchItemFields обновляет только note / sort_order у существующей позиции
func (r *Repository) PatchItemFields(
	ctx context.Context,
	cartID uuid.UUID,
	dishID int,
	note *string,
	sortOrder *int,
) (*repositorymodels.CartItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, patchCartItemFieldsQuery, cartID, dishID, note, sortOrder)
	it, err := scanCartItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cart.ErrCartItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("patch cart item fields: %w", err)
	}
	if _, terr := tx.Exec(ctx, touchCartQuery, cartID); terr != nil {
		return nil, fmt.Errorf("touch cart: %w", terr)
	}
	if cerr := tx.Commit(ctx); cerr != nil {
		return nil, fmt.Errorf("commit tx: %w", cerr)
	}
	return it, nil
}

// FindItem возвращает позицию по (cart_id, dish_id)
func (r *Repository) FindItem(
	ctx context.Context,
	cartID uuid.UUID,
	dishID int,
) (*repositorymodels.CartItem, error) {
	row := r.pool.QueryRow(ctx, findCartItemQuery, cartID, dishID)
	it, err := scanCartItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cart.ErrCartItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find cart item: %w", err)
	}
	return it, nil
}

// DeleteItem удаляет позицию; ErrCartItemNotFound если её не было
func (r *Repository) DeleteItem(ctx context.Context, cartID uuid.UUID, dishID int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, deleteCartItemQuery, cartID, dishID)
	if err != nil {
		return fmt.Errorf("delete cart item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cart.ErrCartItemNotFound
	}
	if _, terr := tx.Exec(ctx, touchCartQuery, cartID); terr != nil {
		return fmt.Errorf("touch cart: %w", terr)
	}
	return tx.Commit(ctx)
}

// DeleteAllItems очищает корзину (без ошибки если она и так пустая)
func (r *Repository) DeleteAllItems(ctx context.Context, cartID uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, derr := tx.Exec(ctx, deleteAllCartItemsQuery, cartID); derr != nil {
		return fmt.Errorf("delete all cart items: %w", derr)
	}
	if _, terr := tx.Exec(ctx, touchCartQuery, cartID); terr != nil {
		return fmt.Errorf("touch cart: %w", terr)
	}
	return tx.Commit(ctx)
}

func scanCart(row pgx.Row) (*repositorymodels.Cart, error) {
	var c repositorymodels.Cart
	if err := row.Scan(&c.ID, &c.UserID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func scanCartItem(row pgx.Row) (*repositorymodels.CartItem, error) {
	var it repositorymodels.CartItem
	if err := row.Scan(&it.CartID, &it.DishID, &it.Quantity, &it.Note, &it.SortOrder, &it.AddedAt); err != nil {
		return nil, err
	}
	return &it, nil
}
