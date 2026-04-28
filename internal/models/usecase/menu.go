package usecase

import (
	"time"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
)

// Cuisine код кухни блюда
type Cuisine string

const (
	// CuisineRussian русская кухня
	CuisineRussian Cuisine = "russian"
	// CuisineItalian итальянская кухня
	CuisineItalian Cuisine = "italian"
	// CuisineJapanese японская кухня
	CuisineJapanese Cuisine = "japanese"
	// CuisineFrench французская кухня
	CuisineFrench Cuisine = "french"
	// CuisineAsian азиатская кухня
	CuisineAsian Cuisine = "asian"
	// CuisineEuropean европейская кухня
	CuisineEuropean Cuisine = "european"
	// CuisineAmerican американская кухня
	CuisineAmerican Cuisine = "american"
)

// IsValid проверяет, что значение допустимо
func (c Cuisine) IsValid() bool {
	switch c {
	case CuisineRussian, CuisineItalian, CuisineJapanese, CuisineFrench,
		CuisineAsian, CuisineEuropean, CuisineAmerican:
		return true
	}
	return false
}

// Category категория меню в доменной форме
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

// CategoryCreate данные для создания категории
type CategoryCreate struct {
	// Name имя
	Name string
	// SortOrder порядок сортировки
	SortOrder int
	// IsAvailable доступна ли в публичной выдаче
	IsAvailable bool
}

// CategoryPatch частичное обновление категории
type CategoryPatch struct {
	// Name имя
	Name *string
	// SortOrder порядок сортировки
	SortOrder *int
	// IsAvailable доступна ли в публичной выдаче
	IsAvailable *bool
}

// Tag тег блюда в доменной форме
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

// TagCreate данные для создания тега
type TagCreate struct {
	// Name имя
	Name string
	// Slug машинное имя
	Slug string
	// Color hex-цвет бэйджа
	Color string
}

// TagPatch частичное обновление тега
type TagPatch struct {
	// Name имя
	Name *string
	// Slug машинное имя
	Slug *string
	// Color hex-цвет бэйджа
	Color *string
}

// Dish блюдо в доменной форме
type Dish struct {
	// ID идентификатор
	ID int
	// Name имя
	Name string
	// Description описание
	Description string
	// Composition состав
	Composition string
	// ImageURL ссылка на изображение
	ImageURL string

	// PriceMinor цена в минорных единицах
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
	Cuisine Cuisine
	// CategoryID идентификатор категории
	CategoryID int

	// Allergens коды аллергенов
	Allergens []string
	// Dietary коды диетических совместимостей
	Dietary []string

	// IsAvailable доступно ли блюдо
	IsAvailable bool

	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time

	// Tags теги, связанные с блюдом
	Tags []Tag
}

// DishCreate данные для создания блюда
type DishCreate struct {
	// Name имя
	Name string
	// Description описание
	Description string
	// Composition состав
	Composition string
	// ImageURL ссылка на изображение
	ImageURL string
	// PriceMinor цена в минорных единицах
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
	Cuisine Cuisine
	// CategoryID идентификатор категории
	CategoryID int
	// Allergens коды аллергенов
	Allergens []string
	// Dietary коды диетических совместимостей
	Dietary []string
	// TagIDs идентификаторы тегов
	TagIDs []int
	// IsAvailable доступно ли блюдо
	IsAvailable bool
}

// DishPatch частичное обновление блюда
type DishPatch struct {
	// Name имя
	Name *string
	// Description описание
	Description *string
	// Composition состав
	Composition *string
	// ImageURL ссылка на изображение
	ImageURL *string
	// PriceMinor цена в минорных единицах
	PriceMinor *int
	// Currency код валюты
	Currency *string
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
	Cuisine *Cuisine
	// CategoryID идентификатор категории
	CategoryID *int
	// Allergens коды аллергенов
	Allergens *[]string
	// Dietary коды диетических совместимостей
	Dietary *[]string
	// TagIDs идентификаторы тегов; nil — не менять связи
	TagIDs *[]int
	// IsAvailable доступно ли блюдо
	IsAvailable *bool
}

// DishFilter фильтр и пагинация для выборки блюд
type DishFilter struct {
	// CategoryID фильтр по категории
	CategoryID *int
	// Available если задано, фильтрует по is_available
	Available *bool
	// Q поисковый запрос
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

// CategoryFromRepository маппит repository.Category в usecase.Category
func CategoryFromRepository(r repositorymodels.Category) Category {
	return Category{
		ID:          r.ID,
		Name:        r.Name,
		SortOrder:   r.SortOrder,
		IsAvailable: r.IsAvailable,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// CategoriesFromRepository маппит slice
func CategoriesFromRepository(rs []repositorymodels.Category) []Category {
	out := make([]Category, len(rs))
	for i, r := range rs {
		out[i] = CategoryFromRepository(r)
	}
	return out
}

// CategoryToRepository маппит usecase.Category в repository.Category
func CategoryToRepository(c *Category) *repositorymodels.Category {
	return &repositorymodels.Category{
		ID:          c.ID,
		Name:        c.Name,
		SortOrder:   c.SortOrder,
		IsAvailable: c.IsAvailable,
	}
}

// TagFromRepository маппит repository.Tag в usecase.Tag
func TagFromRepository(r repositorymodels.Tag) Tag {
	return Tag{
		ID:        r.ID,
		Name:      r.Name,
		Slug:      r.Slug,
		Color:     r.Color,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// TagsFromRepository маппит slice
func TagsFromRepository(rs []repositorymodels.Tag) []Tag {
	out := make([]Tag, len(rs))
	for i, r := range rs {
		out[i] = TagFromRepository(r)
	}
	return out
}

// TagToRepository маппит usecase.Tag в repository.Tag
func TagToRepository(t *Tag) *repositorymodels.Tag {
	return &repositorymodels.Tag{
		ID:    t.ID,
		Name:  t.Name,
		Slug:  t.Slug,
		Color: t.Color,
	}
}

// DishFromRepository маппит repository.Dish в usecase.Dish
func DishFromRepository(r *repositorymodels.Dish) *Dish {
	if r == nil {
		return nil
	}
	return &Dish{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		Composition:    r.Composition,
		ImageURL:       r.ImageURL,
		PriceMinor:     r.PriceMinor,
		Currency:       r.Currency,
		CaloriesKcal:   r.CaloriesKcal,
		ProteinG:       r.ProteinG,
		FatG:           r.FatG,
		CarbsG:         r.CarbsG,
		PortionWeightG: r.PortionWeightG,
		Cuisine:        Cuisine(r.Cuisine),
		CategoryID:     r.CategoryID,
		Allergens:      coalesceSlice(r.Allergens),
		Dietary:        coalesceSlice(r.Dietary),
		IsAvailable:    r.IsAvailable,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		Tags:           TagsFromRepository(r.Tags),
	}
}

// DishesFromRepository маппит slice
func DishesFromRepository(rs []repositorymodels.Dish) []Dish {
	out := make([]Dish, len(rs))
	for i := range rs {
		out[i] = *DishFromRepository(&rs[i])
	}
	return out
}

// DishToRepository маппит usecase.Dish в repository.Dish (без тегов)
func DishToRepository(d *Dish) *repositorymodels.Dish {
	return &repositorymodels.Dish{
		ID:             d.ID,
		Name:           d.Name,
		Description:    d.Description,
		Composition:    d.Composition,
		ImageURL:       d.ImageURL,
		PriceMinor:     d.PriceMinor,
		Currency:       d.Currency,
		CaloriesKcal:   d.CaloriesKcal,
		ProteinG:       d.ProteinG,
		FatG:           d.FatG,
		CarbsG:         d.CarbsG,
		PortionWeightG: d.PortionWeightG,
		Cuisine:        string(d.Cuisine),
		CategoryID:     d.CategoryID,
		Allergens:      coalesceSlice(d.Allergens),
		Dietary:        coalesceSlice(d.Dietary),
		IsAvailable:    d.IsAvailable,
	}
}

// DishFilterToRepository маппит usecase.DishFilter в repository.DishFilter
func DishFilterToRepository(f DishFilter) repositorymodels.DishFilter {
	return repositorymodels.DishFilter{
		CategoryID:       f.CategoryID,
		Available:        f.Available,
		Q:                f.Q,
		ExcludeAllergens: f.ExcludeAllergens,
		Dietary:          f.Dietary,
		TagIDs:           f.TagIDs,
		Limit:            f.Limit,
		Offset:           f.Offset,
	}
}
