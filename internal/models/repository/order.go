package repository

import (
	"time"

	"github.com/google/uuid"
)

// Order заказ в БД (без позиций; позиции — отдельный slice OrderItem)
type Order struct {
	// ID идентификатор заказа
	ID uuid.UUID
	// UserID владелец заказа
	UserID uuid.UUID
	// Status текущий статус (accepted/cooking/ready/in_delivery/closed/cancelled)
	Status string
	// FulfillmentType способ выдачи (delivery/pickup/dine_in)
	FulfillmentType string
	// PaymentMethod способ оплаты (on_delivery/online_stub)
	PaymentMethod string
	// TotalMinor итоговая сумма заказа в копейках (snapshot на момент создания)
	TotalMinor int
	// Currency код валюты (RUB)
	Currency string
	// CustomerFirstName snapshot имени гостя
	CustomerFirstName string
	// CustomerLastName snapshot фамилии
	CustomerLastName string
	// CustomerPhone snapshot телефона
	CustomerPhone string
	// CustomerEmail snapshot email (опц.)
	CustomerEmail *string
	// DeliveryAddress адрес доставки (NULL для pickup/dine_in)
	DeliveryAddress *string
	// DeliveryNotes комментарий курьеру (NULL для pickup/dine_in)
	DeliveryNotes *string
	// Notes комментарий гостя к заказу (опц.)
	Notes *string
	// CreatedAt момент создания
	CreatedAt time.Time
	// UpdatedAt момент последнего изменения
	UpdatedAt time.Time
}

// OrderItem позиция заказа — snapshot блюда на момент создания
type OrderItem struct {
	// OrderID идентификатор заказа
	OrderID uuid.UUID
	// DishID идентификатор блюда (для аналитики)
	DishID int
	// NameSnapshot snapshot названия блюда
	NameSnapshot string
	// PriceMinorSnapshot snapshot цены за единицу в копейках
	PriceMinorSnapshot int
	// Quantity количество (snapshot из cart_items.quantity)
	Quantity int
	// SortOrder сохранённый порядок из корзины
	SortOrder int
}

// OrderFilter фильтр для list-запросов (admin или customer)
type OrderFilter struct {
	// UserID ограничение по владельцу (nil — все)
	UserID *uuid.UUID
	// Status ограничение по статусу (nil — все)
	Status *string
	// From создан не раньше (nil — без ограничения)
	From *time.Time
	// To создан не позже (nil — без ограничения)
	To *time.Time
	// Limit пагинация
	Limit int
	// Offset пагинация
	Offset int
}
