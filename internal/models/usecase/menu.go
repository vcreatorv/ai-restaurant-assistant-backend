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

// CategoryRole роль категории в RAG-пайплайне рекомендаций.
//
// Значения должны совпадать со значениями CHECK-constraint в БД (см. 000009_categories_role.up.sql).
type CategoryRole string

const (
	// CategoryRoleNone категория не участвует в diversify/companion-логике.
	CategoryRoleNone CategoryRole = "none"
	// CategoryRoleMain «основная» категория: на широком запросе сюда добавляем
	// top-1 из непокрытых, чтобы LLM получал разнообразный контекст.
	CategoryRoleMain CategoryRole = "main"
	// CategoryRoleCompanion категория-сопровождение (соус/гарнир/десерт/напиток):
	// по 1 блюду на запрос, пропускается если main уже содержит эту категорию.
	CategoryRoleCompanion CategoryRole = "companion"
)

// Valid возвращает true, если значение роли допустимо в БД (CHECK constraint).
func (r CategoryRole) Valid() bool {
	switch r {
	case CategoryRoleNone, CategoryRoleMain, CategoryRoleCompanion:
		return true
	default:
		return false
	}
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
	// Role роль в RAG-пайплайне: none | main | companion
	Role CategoryRole
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
	// Role роль в RAG-пайплайне; пусто = "none"
	Role CategoryRole
}

// CategoryPatch частичное обновление категории
type CategoryPatch struct {
	// Name имя
	Name *string
	// SortOrder порядок сортировки
	SortOrder *int
	// IsAvailable доступна ли в публичной выдаче
	IsAvailable *bool
	// Role роль в RAG-пайплайне
	Role *CategoryRole
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

	// PairingTags теги-пейринги (с чем сочетается, повод, слот, vibe).
	// Используются для обогащения embed-текста при индексации в Qdrant.
	PairingTags []PairingTag
}

// PairingTag тег-пейринг из контролируемой vocabulary (таблица pairing_tags).
type PairingTag struct {
	// Slug машинный идентификатор (PK)
	Slug string
	// Axis ось: drink | occasion | role | vibe
	Axis string
	// Label человекочитаемая метка для админ UI
	Label string
	// EmbedPhrase фраза, попадающая в embed-текст блюда
	EmbedPhrase string
	// SortOrder порядок в админ UI внутри своей оси
	SortOrder int
	// IsActive если false — тег нельзя присваивать
	IsActive bool
}

// PairingAxis допустимые оси pairing-тегов. Источник истины — CHECK в миграции
// 000012_pairing_tags.up.sql; здесь дублируем как const для валидации в usecase.
type PairingAxis string

// Допустимые значения PairingAxis.
const (
	// PairingAxisDrink — с чем сочетается из напитков (pair_white_wine, …).
	PairingAxisDrink PairingAxis = "drink"
	// PairingAxisOccasion — для какого повода (occasion_date, …).
	PairingAxisOccasion PairingAxis = "occasion"
	// PairingAxisRole — слот в трапезе (role_aperitif, role_main, …).
	PairingAxisRole PairingAxis = "role"
	// PairingAxisVibe — настроение / плотность / температура (vibe_warming, …).
	PairingAxisVibe PairingAxis = "vibe"
)

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
	// PairingTagSlugs идентификаторы pairing-тегов (slug'и из pairing_tags)
	PairingTagSlugs []string
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
	// PairingTagSlugs идентификаторы pairing-тегов; nil — не менять связи
	PairingTagSlugs *[]string
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

// CategoryFromRepository маппит repository.Category в usecase.Category.
//
// Пустая строка из БД (для случаев pre-migration данных без role) маппится в CategoryRoleNone.
func CategoryFromRepository(r repositorymodels.Category) Category {
	role := CategoryRole(r.Role)
	if role == "" {
		role = CategoryRoleNone
	}
	return Category{
		ID:          r.ID,
		Name:        r.Name,
		SortOrder:   r.SortOrder,
		IsAvailable: r.IsAvailable,
		Role:        role,
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

// CategoryToRepository маппит usecase.Category в repository.Category.
//
// Пустая Role нормализуется в "none", чтобы не нарушить NOT NULL / CHECK в БД.
func CategoryToRepository(c *Category) *repositorymodels.Category {
	role := c.Role
	if role == "" {
		role = CategoryRoleNone
	}
	return &repositorymodels.Category{
		ID:          c.ID,
		Name:        c.Name,
		SortOrder:   c.SortOrder,
		IsAvailable: c.IsAvailable,
		Role:        string(role),
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
		PairingTags:    PairingTagsFromRepository(r.PairingTags),
	}
}

// PairingTagFromRepository маппит repository.PairingTag в usecase.PairingTag
func PairingTagFromRepository(r repositorymodels.PairingTag) PairingTag {
	return PairingTag{
		Slug:        r.Slug,
		Axis:        r.Axis,
		Label:       r.Label,
		EmbedPhrase: r.EmbedPhrase,
		SortOrder:   r.SortOrder,
		IsActive:    r.IsActive,
	}
}

// PairingTagsFromRepository маппит slice repository.PairingTag в usecase.PairingTag
func PairingTagsFromRepository(rs []repositorymodels.PairingTag) []PairingTag {
	if len(rs) == 0 {
		return nil
	}
	out := make([]PairingTag, len(rs))
	for i, r := range rs {
		out[i] = PairingTagFromRepository(r)
	}
	return out
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
