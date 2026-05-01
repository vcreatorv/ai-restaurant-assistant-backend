// Package cart описывает доменные интерфейсы и ошибки фичи «корзина».
package cart

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	"github.com/google/uuid"
)

var (
	// ErrCartItemNotFound позиция не найдена в корзине
	ErrCartItemNotFound = errors.New("cart item not found")
	// ErrDishNotFound блюдо не существует
	ErrDishNotFound = errors.New("dish not found")
	// ErrDishUnavailable блюдо в стоп-листе (is_available=false) — нельзя добавить
	ErrDishUnavailable = errors.New("dish unavailable")
	// ErrInvalidQuantity quantity вне диапазона 1..50
	ErrInvalidQuantity = errors.New("invalid quantity")
)

// QuantityMin минимально допустимое количество одной позиции
const QuantityMin = 1

// QuantityMax максимально допустимое количество одной позиции
const QuantityMax = 50

// Usecase сценарии работы с корзиной
type Usecase interface {
	// Get возвращает текущую корзину пользователя; если её ещё нет — создаёт пустую.
	// CartView включает актуальные данные блюд + предупреждения по стоп-листу.
	Get(ctx context.Context, userID uuid.UUID) (*usecasemodels.CartView, error)
	// AddItem добавляет позицию (или увеличивает quantity, если позиция уже есть).
	// Если в результате сумма quantity > QuantityMax — ErrInvalidQuantity.
	AddItem(ctx context.Context, userID uuid.UUID, req usecasemodels.CartItemAdd) (*usecasemodels.CartView, error)
	// PatchItem меняет quantity / note / sort_order у позиции
	PatchItem(
		ctx context.Context,
		userID uuid.UUID,
		dishID int,
		patch usecasemodels.CartItemPatch,
	) (*usecasemodels.CartView, error)
	// RemoveItem удаляет позицию по dish_id
	RemoveItem(ctx context.Context, userID uuid.UUID, dishID int) error
	// Clear удаляет все позиции корзины
	Clear(ctx context.Context, userID uuid.UUID) error
}

// Repository хранилище корзин
type Repository interface {
	// FindOrCreateCart возвращает корзину пользователя; создаёт пустую если её нет
	FindOrCreateCart(ctx context.Context, userID uuid.UUID) (*repositorymodels.Cart, error)
	// ListItems возвращает позиции корзины, отсортированные (sort_order, added_at)
	ListItems(ctx context.Context, cartID uuid.UUID) ([]repositorymodels.CartItem, error)
	// UpsertItem вставляет позицию или увеличивает quantity, если (cart_id, dish_id) уже есть.
	// Возвращает финальную позицию после операции.
	UpsertItem(
		ctx context.Context,
		cartID uuid.UUID,
		dishID, quantityDelta int,
		note *string,
	) (*repositorymodels.CartItem, error)
	// SetItemQuantity жёстко выставляет quantity (для PATCH без суммирования)
	SetItemQuantity(ctx context.Context, cartID uuid.UUID, dishID, quantity int) (*repositorymodels.CartItem, error)
	// PatchItemFields обновляет note / sort_order; quantity не трогает (отдельным методом)
	PatchItemFields(
		ctx context.Context,
		cartID uuid.UUID,
		dishID int,
		note *string,
		sortOrder *int,
	) (*repositorymodels.CartItem, error)
	// FindItem возвращает позицию по (cart_id, dish_id) или ErrCartItemNotFound
	FindItem(ctx context.Context, cartID uuid.UUID, dishID int) (*repositorymodels.CartItem, error)
	// DeleteItem удаляет позицию; если позиции не было — возвращает ErrCartItemNotFound
	DeleteItem(ctx context.Context, cartID uuid.UUID, dishID int) error
	// DeleteAllItems очищает корзину
	DeleteAllItems(ctx context.Context, cartID uuid.UUID) error
}

// UUIDGen генератор UUID для cart-id
type UUIDGen interface {
	// New генерирует новый UUID
	New() uuid.UUID
}
