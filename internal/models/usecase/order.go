package usecase

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus статус заказа
type OrderStatus string

// Жизненный цикл заказа: accepted → cooking → ready → in_delivery → closed; cancelled — терминал.
const (
	// OrderStatusAccepted принят (создан гостем, кухня видит)
	OrderStatusAccepted OrderStatus = "accepted"
	// OrderStatusCooking в работе у кухни
	OrderStatusCooking OrderStatus = "cooking"
	// OrderStatusReady готов к выдаче / доставке
	OrderStatusReady OrderStatus = "ready"
	// OrderStatusInDelivery в пути к гостю
	OrderStatusInDelivery OrderStatus = "in_delivery"
	// OrderStatusClosed заказ закрыт (гость получил)
	OrderStatusClosed OrderStatus = "closed"
	// OrderStatusCancelled отменён admin'ом
	OrderStatusCancelled OrderStatus = "cancelled"
)

// FulfillmentType способ выдачи заказа
type FulfillmentType string

const (
	// FulfillmentDelivery доставка курьером по адресу
	FulfillmentDelivery FulfillmentType = "delivery"
	// FulfillmentPickup самовывоз из ресторана
	FulfillmentPickup FulfillmentType = "pickup"
	// FulfillmentDineIn в зале ресторана
	FulfillmentDineIn FulfillmentType = "dine_in"
)

// PaymentMethod способ оплаты
type PaymentMethod string

const (
	// PaymentOnDelivery оплата при получении
	PaymentOnDelivery PaymentMethod = "on_delivery"
	// PaymentOnlineStub заглушка под будущую онлайн-оплату (ЮKassa и т.п.)
	PaymentOnlineStub PaymentMethod = "online_stub"
)

// Order представление заказа для API: основные поля + items
type Order struct {
	// ID идентификатор заказа
	ID uuid.UUID
	// UserID владелец
	UserID uuid.UUID
	// Status текущий статус
	Status OrderStatus
	// FulfillmentType способ выдачи
	FulfillmentType FulfillmentType
	// PaymentMethod способ оплаты
	PaymentMethod PaymentMethod
	// TotalMinor сумма заказа в копейках
	TotalMinor int
	// Currency валюта (RUB)
	Currency string
	// CustomerFirstName snapshot имени
	CustomerFirstName string
	// CustomerLastName snapshot фамилии
	CustomerLastName string
	// CustomerPhone snapshot телефона
	CustomerPhone string
	// CustomerEmail snapshot email (опц.)
	CustomerEmail *string
	// DeliveryAddress адрес доставки (только для FulfillmentDelivery)
	DeliveryAddress *string
	// DeliveryNotes комментарий курьеру
	DeliveryNotes *string
	// Notes комментарий гостя к заказу
	Notes *string
	// Items позиции заказа
	Items []OrderItem
	// CreatedAt момент создания
	CreatedAt time.Time
	// UpdatedAt момент последнего изменения
	UpdatedAt time.Time
}

// OrderItem позиция заказа — snapshot названия и цены
type OrderItem struct {
	// DishID идентификатор блюда (для аналитики; всегда non-nil на B2 — блюда не удаляются физически)
	DishID *int
	// DishName snapshot названия
	DishName string
	// DishPriceMinor snapshot цены за единицу в копейках
	DishPriceMinor int
	// Quantity количество в заказе
	Quantity int
	// LineTotalMinor DishPriceMinor * Quantity
	LineTotalMinor int
	// SortOrder сохранённый порядок из корзины
	SortOrder int
}

// CreateOrderRequest входные данные оформления заказа из текущей корзины
type CreateOrderRequest struct {
	// FulfillmentType способ выдачи
	FulfillmentType FulfillmentType
	// PaymentMethod способ оплаты
	PaymentMethod PaymentMethod
	// DeliveryAddress адрес (обязателен при FulfillmentDelivery)
	DeliveryAddress *string
	// DeliveryNotes комментарий курьеру
	DeliveryNotes *string
	// Notes комментарий гостя
	Notes *string
}

// OrderListFilter фильтр выдачи заказов
type OrderListFilter struct {
	// Status фильтр по статусу
	Status *OrderStatus
	// From создан не раньше
	From *time.Time
	// To создан не позже
	To *time.Time
	// Limit пагинация
	Limit int
	// Offset пагинация
	Offset int
}
