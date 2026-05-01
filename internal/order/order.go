// Package order описывает доменные интерфейсы и ошибки фичи «заказы».
package order

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	"github.com/google/uuid"
)

var (
	// ErrOrderNotFound заказ не найден
	ErrOrderNotFound = errors.New("order not found")
	// ErrOrderForbidden заказ принадлежит другому пользователю
	ErrOrderForbidden = errors.New("order does not belong to this user")
	// ErrCartEmpty корзина пуста — нечего оформлять
	ErrCartEmpty = errors.New("cart is empty")
	// ErrProfileIncomplete не заполнены first_name / last_name / phone в профиле
	ErrProfileIncomplete = errors.New("profile is incomplete")
	// ErrDeliveryAddressRequired для fulfillment_type=delivery нужен delivery_address
	ErrDeliveryAddressRequired = errors.New("delivery address is required")
	// ErrCheckoutItemsUnavailable в корзине есть позиции из стоп-листа — оформить нельзя.
	// IDs недоступных блюд можно достать через UnavailableDishIDs(err).
	ErrCheckoutItemsUnavailable = errors.New("some items are unavailable")
	// ErrInvalidFulfillmentType невалидное значение fulfillment_type
	ErrInvalidFulfillmentType = errors.New("invalid fulfillment_type")
	// ErrInvalidPaymentMethod невалидное значение payment_method
	ErrInvalidPaymentMethod = errors.New("invalid payment_method")
	// ErrInvalidStatusTransition недопустимый переход статуса
	// (например из закрытого/отменённого, или нелинейный пропуск этапов)
	ErrInvalidStatusTransition = errors.New("invalid order status transition")
	// ErrInvalidStatus невалидное значение status
	ErrInvalidStatus = errors.New("invalid order status")
)

// CheckoutItemsUnavailableError расширяет ErrCheckoutItemsUnavailable списком id.
// errors.Is(err, ErrCheckoutItemsUnavailable) → true; чтобы достать DishIDs — errors.As.
type CheckoutItemsUnavailableError struct {
	DishIDs []int
}

// Error возвращает текст ошибки
func (e *CheckoutItemsUnavailableError) Error() string {
	return ErrCheckoutItemsUnavailable.Error()
}

// Unwrap позволяет errors.Is(err, ErrCheckoutItemsUnavailable)
func (e *CheckoutItemsUnavailableError) Unwrap() error {
	return ErrCheckoutItemsUnavailable
}

// Usecase сценарии работы с заказами
type Usecase interface {
	// Create оформляет заказ из текущей корзины пользователя; чистит корзину.
	// Может вернуть: ErrCartEmpty, ErrProfileIncomplete, ErrDeliveryAddressRequired,
	// *CheckoutItemsUnavailableError, ErrInvalidFulfillmentType, ErrInvalidPaymentMethod.
	Create(
		ctx context.Context,
		userID uuid.UUID,
		req usecasemodels.CreateOrderRequest,
	) (*usecasemodels.Order, error)
	// List возвращает заказы пользователя с фильтром и пагинацией
	List(
		ctx context.Context,
		userID uuid.UUID,
		filter usecasemodels.OrderListFilter,
	) ([]usecasemodels.Order, int, error)
	// Get возвращает заказ по id; проверяет, что он принадлежит userID (иначе ErrOrderForbidden)
	Get(ctx context.Context, userID, orderID uuid.UUID) (*usecasemodels.Order, error)

	// AdminList возвращает заказы по фильтру (без ограничения по userID — admin видит всё)
	AdminList(
		ctx context.Context,
		filter usecasemodels.OrderListFilter,
		userID *uuid.UUID,
	) ([]usecasemodels.Order, int, error)
	// AdminGet возвращает заказ по id (без проверки владельца)
	AdminGet(ctx context.Context, orderID uuid.UUID) (*usecasemodels.Order, error)
	// AdminUpdateStatus меняет статус заказа с проверкой state-machine.
	// При недопустимом переходе — ErrInvalidStatusTransition.
	AdminUpdateStatus(
		ctx context.Context,
		orderID uuid.UUID,
		newStatus usecasemodels.OrderStatus,
	) (*usecasemodels.Order, error)
}

// Repository хранилище заказов
type Repository interface {
	// CreateOrder вставляет заказ + позиции в одной транзакции
	CreateOrder(ctx context.Context, o *repositorymodels.Order, items []repositorymodels.OrderItem) error
	// FindOrderByID возвращает заказ + позиции; ErrOrderNotFound если нет
	FindOrderByID(
		ctx context.Context,
		orderID uuid.UUID,
	) (*repositorymodels.Order, []repositorymodels.OrderItem, error)
	// ListOrders возвращает заказы по фильтру + total. Позиции для каждого заказа
	// загружаются batch'ем (LoadOrderItems на возвращённых ID).
	ListOrders(
		ctx context.Context,
		filter repositorymodels.OrderFilter,
	) (orders []repositorymodels.Order, total int, err error)
	// LoadOrderItems batch-загрузка позиций для списка заказов;
	// возвращает map orderID → []OrderItem
	LoadOrderItems(
		ctx context.Context,
		orderIDs []uuid.UUID,
	) (map[uuid.UUID][]repositorymodels.OrderItem, error)
	// UpdateOrderStatus обновляет status и updated_at; возвращает обновлённую запись.
	// ErrOrderNotFound если заказа нет.
	UpdateOrderStatus(
		ctx context.Context,
		orderID uuid.UUID,
		newStatus string,
	) (*repositorymodels.Order, error)
}

// UUIDGen генератор UUID для order-id
type UUIDGen interface {
	// New генерирует новый UUID
	New() uuid.UUID
}
