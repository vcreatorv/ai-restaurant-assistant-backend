package repository

import "time"

// Category категория меню в storage-форме
type Category struct {
	// ID идентификатор
	ID int
	// Name имя
	Name string
	// SortOrder порядок сортировки
	SortOrder int
	// IsAvailable доступна ли в публичной выдаче
	IsAvailable bool
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}

// Tag тег блюда в storage-форме
type Tag struct {
	// ID идентификатор
	ID int
	// Name имя
	Name string
	// Slug машинное имя
	Slug string
	// Color hex-цвет бэйджа
	Color string
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}

// Dish блюдо в storage-форме
type Dish struct {
	// ID идентификатор
	ID int
	// Name имя
	Name string
	// Description описание
	Description string
	// Composition состав в виде свободного текста
	Composition string
	// ImageURL ссылка на изображение
	ImageURL string

	// PriceMinor цена в минорных единицах валюты
	PriceMinor int
	// Currency код валюты
	Currency string

	// CaloriesKcal калории
	CaloriesKcal *int
	// ProteinG белки
	ProteinG *float64
	// FatG жиры
	FatG *float64
	// CarbsG углеводы
	CarbsG *float64
	// PortionWeightG вес порции
	PortionWeightG *int

	// Cuisine код кухни
	Cuisine string
	// CategoryID идентификатор категории
	CategoryID int

	// Allergens коды аллергенов
	Allergens []string
	// Dietary коды диетических совместимостей
	Dietary []string

	// IsAvailable доступно ли блюдо в публичной выдаче
	IsAvailable bool

	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time

	// Tags теги, связанные с блюдом
	Tags []Tag
}

// DishFilter фильтр и пагинация для выборки блюд
type DishFilter struct {
	// CategoryID фильтр по категории
	CategoryID *int
	// Available если задано, фильтрует по is_available
	Available *bool
	// Q поисковый запрос (ILIKE по name)
	Q string
	// ExcludeAllergens исключить блюда, в которых есть хотя бы один из этих аллергенов
	ExcludeAllergens []string
	// Dietary блюда должны включать все указанные диетические совместимости
	Dietary []string
	// TagIDs блюда должны быть привязаны хотя бы к одному из тегов
	TagIDs []int
	// Limit лимит записей
	Limit int
	// Offset смещение
	Offset int
}
