package repository

import (
	"time"

	"github.com/google/uuid"
)

// Cart активная корзина пользователя в БД (без позиций).
// Один cart на user_id (UNIQUE constraint на user_id).
type Cart struct {
	// ID идентификатор корзины
	ID uuid.UUID
	// UserID владелец корзины
	UserID uuid.UUID
	// CreatedAt момент создания
	CreatedAt time.Time
	// UpdatedAt момент последнего изменения
	UpdatedAt time.Time
}

// CartItem позиция в корзине.
// Уникальность по (cart_id, dish_id) — добавление того же блюда увеличивает quantity.
type CartItem struct {
	// CartID идентификатор корзины
	CartID uuid.UUID
	// DishID идентификатор блюда
	DishID int
	// Quantity количество (1..50)
	Quantity int
	// Note пользовательская заметка к позиции (опц.)
	Note *string
	// SortOrder ручной порядок сортировки (по умолчанию 0)
	SortOrder int
	// AddedAt момент добавления
	AddedAt time.Time
}
