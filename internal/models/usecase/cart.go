package usecase

import (
	"time"

	"github.com/google/uuid"
)

// CartView представление корзины для API: позиции с актуальной информацией о блюде
// (имя, цена, доступность) + total + предупреждения по спорным позициям.
type CartView struct {
	// Items позиции корзины с раскрытыми данными блюд
	Items []CartItemView
	// TotalMinor сумма line_total_minor по доступным (available=true) позициям, в копейках
	TotalMinor int
	// Currency код валюты (одинаковый для всех позиций; на MVP всегда RUB)
	Currency string
	// Warnings предупреждения для UI (стоп-лист, изменение цены и т.п.)
	Warnings []CartWarning
}

// CartItemView позиция корзины, обогащённая актуальными данными блюда из БД.
// Цена показывается current — если изменилась с момента добавления, в Warnings
// будет соответствующая запись.
type CartItemView struct {
	// DishID идентификатор блюда
	DishID int
	// Name актуальное название блюда
	Name string
	// PriceMinor актуальная цена за единицу в копейках
	PriceMinor int
	// Quantity количество в корзине
	Quantity int
	// Note заметка к позиции
	Note *string
	// SortOrder ручной порядок
	SortOrder int
	// AddedAt момент добавления
	AddedAt time.Time
	// Available блюдо доступно к заказу (есть в меню и is_available=true)
	Available bool
	// LineTotalMinor PriceMinor * Quantity (показывается даже если available=false)
	LineTotalMinor int
}

// CartWarning код предупреждения для одной позиции.
// Возможные codes: dish_unavailable (блюдо в стоп-листе или удалено).
type CartWarning struct {
	// Code машинно-читаемый код предупреждения
	Code string
	// DishID идентификатор затронутого блюда
	DishID int
}

// Коды предупреждений
const (
	// CartWarningDishUnavailable блюдо больше недоступно (is_available=false или удалено)
	CartWarningDishUnavailable = "dish_unavailable"
)

// CartSource источник добавления блюда в корзину.
//
// Влияет только на запись в cart_additions для аналитики; на саму корзину не влияет.
// Значения должны совпадать с CHECK constraint в БД (см. 000011_suggestions_and_cart_additions).
type CartSource string

const (
	// CartSourceChat — добавление со страницы чата (карточка блюда в ответе ассистента).
	CartSourceChat CartSource = "chat"
	// CartSourceMenu — добавление со страницы публичного меню.
	CartSourceMenu CartSource = "menu"
	// CartSourceCart — изменение со страницы корзины (резерв на будущее).
	CartSourceCart CartSource = "cart"
	// CartSourceOther — fallback для запросов без поля source (старые клиенты).
	CartSourceOther CartSource = "other"
)

// Valid возвращает true, если значение допустимо в БД.
func (s CartSource) Valid() bool {
	switch s {
	case CartSourceChat, CartSourceMenu, CartSourceCart, CartSourceOther:
		return true
	default:
		return false
	}
}

// CartItemAdd параметры добавления позиции в корзину
type CartItemAdd struct {
	// DishID id блюда
	DishID int
	// Quantity сколько добавить (если позиция уже есть — сумируется)
	Quantity int
	// Note заметка (опц.)
	Note *string
	// Source источник добавления (chat | menu | cart | other); пустая → other.
	// Не влияет на саму корзину, нужен только для аналитики в cart_additions.
	Source CartSource
	// MessageID id assistant-сообщения, из которого взято блюдо (если source='chat').
	// Опционально: помогает связать клик в чате с конкретной рекомендацией.
	MessageID *uuid.UUID
}

// CartItemPatch патч позиции; nil-поля не трогаются
type CartItemPatch struct {
	// Quantity новое количество (>= 1, иначе — удалить через DELETE)
	Quantity *int
	// Note заметка (nil не трогает; чтобы стереть — передать pointer на пустую строку)
	Note *string
	// SortOrder ручной порядок
	SortOrder *int
}
