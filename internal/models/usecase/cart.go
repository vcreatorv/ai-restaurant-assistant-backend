package usecase

import "time"

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

// CartItemAdd параметры добавления позиции в корзину
type CartItemAdd struct {
	// DishID id блюда
	DishID int
	// Quantity сколько добавить (если позиция уже есть — сумируется)
	Quantity int
	// Note заметка (опц.)
	Note *string
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
