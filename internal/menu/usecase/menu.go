package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// ListCategories возвращает все доступные категории
func (uc *menuUsecase) ListCategories(ctx context.Context) ([]usecasemodels.Category, error) {
	rs, err := uc.repo.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return usecasemodels.CategoriesFromRepository(rs), nil
}

// CreateCategory создаёт категорию
func (uc *menuUsecase) CreateCategory(
	ctx context.Context,
	c usecasemodels.CategoryCreate,
) (*usecasemodels.Category, error) {
	r := &usecasemodels.Category{
		Name:        c.Name,
		SortOrder:   c.SortOrder,
		IsAvailable: c.IsAvailable,
	}
	repo := usecasemodels.CategoryToRepository(r)
	if err := uc.repo.CreateCategory(ctx, repo); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	out := usecasemodels.CategoryFromRepository(*repo)
	return &out, nil
}

// UpdateCategory обновляет категорию
func (uc *menuUsecase) UpdateCategory(
	ctx context.Context,
	id int,
	p usecasemodels.CategoryPatch,
) (*usecasemodels.Category, error) {
	raw, err := uc.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	c := usecasemodels.CategoryFromRepository(*raw)
	if p.Name != nil {
		c.Name = *p.Name
	}
	if p.SortOrder != nil {
		c.SortOrder = *p.SortOrder
	}
	if p.IsAvailable != nil {
		c.IsAvailable = *p.IsAvailable
	}
	if err := uc.repo.UpdateCategory(ctx, usecasemodels.CategoryToRepository(&c)); err != nil {
		return nil, fmt.Errorf("update category: %w", err)
	}
	return &c, nil
}

// DeleteCategory удаляет категорию
func (uc *menuUsecase) DeleteCategory(ctx context.Context, id int) error {
	return uc.repo.DeleteCategory(ctx, id)
}

// ListTags возвращает все теги
func (uc *menuUsecase) ListTags(ctx context.Context) ([]usecasemodels.Tag, error) {
	rs, err := uc.repo.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return usecasemodels.TagsFromRepository(rs), nil
}

// CreateTag создаёт тег
func (uc *menuUsecase) CreateTag(ctx context.Context, t usecasemodels.TagCreate) (*usecasemodels.Tag, error) {
	tag := &usecasemodels.Tag{
		Name:  t.Name,
		Slug:  t.Slug,
		Color: t.Color,
	}
	repo := usecasemodels.TagToRepository(tag)
	if err := uc.repo.CreateTag(ctx, repo); err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	out := usecasemodels.TagFromRepository(*repo)
	return &out, nil
}

// UpdateTag обновляет тег
func (uc *menuUsecase) UpdateTag(ctx context.Context, id int, p usecasemodels.TagPatch) (*usecasemodels.Tag, error) {
	raw, err := uc.repo.FindTagByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t := usecasemodels.TagFromRepository(*raw)
	if p.Name != nil {
		t.Name = *p.Name
	}
	if p.Slug != nil {
		t.Slug = *p.Slug
	}
	if p.Color != nil {
		t.Color = *p.Color
	}
	if err := uc.repo.UpdateTag(ctx, usecasemodels.TagToRepository(&t)); err != nil {
		return nil, fmt.Errorf("update tag: %w", err)
	}
	return &t, nil
}

// DeleteTag удаляет тег
func (uc *menuUsecase) DeleteTag(ctx context.Context, id int) error {
	return uc.repo.DeleteTag(ctx, id)
}

// ListDishes возвращает блюда с фильтрами и пагинацией
func (uc *menuUsecase) ListDishes(ctx context.Context, f usecasemodels.DishFilter) ([]usecasemodels.Dish, int, error) {
	rs, total, err := uc.repo.ListDishes(ctx, usecasemodels.DishFilterToRepository(f))
	if err != nil {
		return nil, 0, fmt.Errorf("list dishes: %w", err)
	}
	return usecasemodels.DishesFromRepository(rs), total, nil
}

// GetDish возвращает блюдо по идентификатору
func (uc *menuUsecase) GetDish(ctx context.Context, id int) (*usecasemodels.Dish, error) {
	raw, err := uc.repo.FindDishByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return usecasemodels.DishFromRepository(raw), nil
}

// GetDishesByIDs возвращает блюда по списку идентификаторов
func (uc *menuUsecase) GetDishesByIDs(
	ctx context.Context,
	ids []int,
) ([]usecasemodels.Dish, error) {
	if len(ids) == 0 {
		return []usecasemodels.Dish{}, nil
	}
	raws, err := uc.repo.FindDishesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find dishes by ids: %w", err)
	}
	return usecasemodels.DishesFromRepository(raws), nil
}

// CreateDish создаёт блюдо
func (uc *menuUsecase) CreateDish(ctx context.Context, d usecasemodels.DishCreate) (*usecasemodels.Dish, error) {
	if !d.Cuisine.IsValid() {
		return nil, menu.ErrInvalidCuisine
	}
	if d.Currency == "" {
		d.Currency = "RUB"
	}
	dish := &usecasemodels.Dish{
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
		Cuisine:        d.Cuisine,
		CategoryID:     d.CategoryID,
		Allergens:      d.Allergens,
		Dietary:        d.Dietary,
		IsAvailable:    d.IsAvailable,
	}
	repo := usecasemodels.DishToRepository(dish)
	if err := uc.repo.CreateDish(ctx, repo, d.TagIDs); err != nil {
		return nil, fmt.Errorf("create dish: %w", err)
	}
	return usecasemodels.DishFromRepository(repo), nil
}

// UpdateDish обновляет блюдо
func (uc *menuUsecase) UpdateDish(ctx context.Context, id int, p usecasemodels.DishPatch) (*usecasemodels.Dish, error) {
	raw, err := uc.repo.FindDishByID(ctx, id)
	if err != nil {
		return nil, err
	}
	d := usecasemodels.DishFromRepository(raw)
	applyDishPatch(d, p)
	if !d.Cuisine.IsValid() {
		return nil, menu.ErrInvalidCuisine
	}
	repo := usecasemodels.DishToRepository(d)
	repo.ID = id
	var tagIDs []int
	if p.TagIDs != nil {
		tagIDs = *p.TagIDs
	}
	if err := uc.repo.UpdateDish(ctx, repo, tagIDs); err != nil {
		return nil, fmt.Errorf("update dish: %w", err)
	}
	return uc.GetDish(ctx, id)
}

// DeleteDish помечает блюдо недоступным
func (uc *menuUsecase) DeleteDish(ctx context.Context, id int) error {
	return uc.repo.SetDishAvailability(ctx, id, false)
}

// UploadDishImage заливает картинку и сохраняет URL в блюде
func (uc *menuUsecase) UploadDishImage(
	ctx context.Context,
	id int,
	src menu.DishImageSource,
) (*usecasemodels.Dish, error) {
	raw, err := uc.repo.FindDishByID(ctx, id)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("dishes/%d-%d.%s", id, time.Now().UnixNano(), src.Ext)
	url, err := uc.storage.Upload(ctx, key, src.ContentType, src.Body, src.Size)
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}
	d := usecasemodels.DishFromRepository(raw)
	d.ImageURL = url
	repo := usecasemodels.DishToRepository(d)
	repo.ID = id
	if err := uc.repo.UpdateDish(ctx, repo, nil); err != nil {
		return nil, fmt.Errorf("persist image_url: %w", err)
	}
	return uc.GetDish(ctx, id)
}

func applyDishPatch(d *usecasemodels.Dish, p usecasemodels.DishPatch) {
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.Description != nil {
		d.Description = *p.Description
	}
	if p.Composition != nil {
		d.Composition = *p.Composition
	}
	if p.ImageURL != nil {
		d.ImageURL = *p.ImageURL
	}
	if p.PriceMinor != nil {
		d.PriceMinor = *p.PriceMinor
	}
	if p.Currency != nil {
		d.Currency = *p.Currency
	}
	if p.CaloriesKcal != nil {
		d.CaloriesKcal = p.CaloriesKcal
	}
	if p.ProteinG != nil {
		d.ProteinG = p.ProteinG
	}
	if p.FatG != nil {
		d.FatG = p.FatG
	}
	if p.CarbsG != nil {
		d.CarbsG = p.CarbsG
	}
	if p.PortionWeightG != nil {
		d.PortionWeightG = p.PortionWeightG
	}
	if p.Cuisine != nil {
		d.Cuisine = *p.Cuisine
	}
	if p.CategoryID != nil {
		d.CategoryID = *p.CategoryID
	}
	if p.Allergens != nil {
		d.Allergens = *p.Allergens
	}
	if p.Dietary != nil {
		d.Dietary = *p.Dietary
	}
	if p.IsAvailable != nil {
		d.IsAvailable = *p.IsAvailable
	}
}
