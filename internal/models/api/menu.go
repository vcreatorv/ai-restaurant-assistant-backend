package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// CategoryFromUsecase маппит usecase.Category в api.Category
func CategoryFromUsecase(c usecasemodels.Category) Category {
	return Category{
		Id:          c.ID,
		Name:        c.Name,
		SortOrder:   c.SortOrder,
		IsAvailable: c.IsAvailable,
	}
}

// CategoryListFromUsecase маппит slice в api.CategoryList
func CategoryListFromUsecase(cs []usecasemodels.Category) CategoryList {
	items := make([]Category, len(cs))
	for i, c := range cs {
		items[i] = CategoryFromUsecase(c)
	}
	return CategoryList{Items: items}
}

// CreateCategoryRequestToUsecase маппит api.CreateCategoryRequest в usecase.CategoryCreate
func CreateCategoryRequestToUsecase(req CreateCategoryRequest) usecasemodels.CategoryCreate {
	c := usecasemodels.CategoryCreate{
		Name:        req.Name,
		SortOrder:   0,
		IsAvailable: true,
	}
	if req.SortOrder != nil {
		c.SortOrder = *req.SortOrder
	}
	if req.IsAvailable != nil {
		c.IsAvailable = *req.IsAvailable
	}
	return c
}

// PatchCategoryRequestToUsecase маппит api.PatchCategoryRequest в usecase.CategoryPatch
func PatchCategoryRequestToUsecase(req PatchCategoryRequest) usecasemodels.CategoryPatch {
	return usecasemodels.CategoryPatch{
		Name:        req.Name,
		SortOrder:   req.SortOrder,
		IsAvailable: req.IsAvailable,
	}
}

// TagFromUsecase маппит usecase.Tag в api.Tag
func TagFromUsecase(t usecasemodels.Tag) Tag {
	return Tag{
		Id:    t.ID,
		Name:  t.Name,
		Slug:  t.Slug,
		Color: t.Color,
	}
}

// TagListFromUsecase маппит slice в api.TagList
func TagListFromUsecase(ts []usecasemodels.Tag) TagList {
	items := make([]Tag, len(ts))
	for i, t := range ts {
		items[i] = TagFromUsecase(t)
	}
	return TagList{Items: items}
}

// CreateTagRequestToUsecase маппит api.CreateTagRequest в usecase.TagCreate
func CreateTagRequestToUsecase(req CreateTagRequest) usecasemodels.TagCreate {
	t := usecasemodels.TagCreate{
		Name:  req.Name,
		Slug:  req.Slug,
		Color: "#888888",
	}
	if req.Color != nil {
		t.Color = *req.Color
	}
	return t
}

// PatchTagRequestToUsecase маппит api.PatchTagRequest в usecase.TagPatch
func PatchTagRequestToUsecase(req PatchTagRequest) usecasemodels.TagPatch {
	return usecasemodels.TagPatch{
		Name:  req.Name,
		Slug:  req.Slug,
		Color: req.Color,
	}
}

// DishFromUsecase маппит usecase.Dish в api.Dish
func DishFromUsecase(d *usecasemodels.Dish) Dish {
	out := Dish{
		Id:             d.ID,
		Name:           d.Name,
		Description:    d.Description,
		Composition:    d.Composition,
		ImageUrl:       d.ImageURL,
		PriceMinor:     d.PriceMinor,
		Currency:       d.Currency,
		CategoryId:     d.CategoryID,
		Cuisine:        Cuisine(d.Cuisine),
		Allergens:      coalesceSlice(d.Allergens),
		Dietary:        coalesceSlice(d.Dietary),
		IsAvailable:    d.IsAvailable,
		CaloriesKcal:   d.CaloriesKcal,
		PortionWeightG: d.PortionWeightG,
		Tags:           tagsToAPI(d.Tags),
	}
	if d.ProteinG != nil {
		v := float32(*d.ProteinG)
		out.ProteinG = &v
	}
	if d.FatG != nil {
		v := float32(*d.FatG)
		out.FatG = &v
	}
	if d.CarbsG != nil {
		v := float32(*d.CarbsG)
		out.CarbsG = &v
	}
	return out
}

// DishListFromUsecase маппит slice в api.DishList
func DishListFromUsecase(ds []usecasemodels.Dish, total, limit, offset int) DishList {
	items := make([]Dish, len(ds))
	for i := range ds {
		items[i] = DishFromUsecase(&ds[i])
	}
	return DishList{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// CreateDishRequestToUsecase маппит api.CreateDishRequest в usecase.DishCreate
func CreateDishRequestToUsecase(req CreateDishRequest) usecasemodels.DishCreate {
	d := usecasemodels.DishCreate{
		Name:           req.Name,
		PriceMinor:     req.PriceMinor,
		Currency:       "RUB",
		Cuisine:        usecasemodels.Cuisine(req.Cuisine),
		CategoryID:     req.CategoryId,
		IsAvailable:    true,
		CaloriesKcal:   req.CaloriesKcal,
		PortionWeightG: req.PortionWeightG,
	}
	if req.Description != nil {
		d.Description = *req.Description
	}
	if req.Composition != nil {
		d.Composition = *req.Composition
	}
	if req.ImageUrl != nil {
		d.ImageURL = *req.ImageUrl
	}
	if req.Currency != nil {
		d.Currency = *req.Currency
	}
	if req.IsAvailable != nil {
		d.IsAvailable = *req.IsAvailable
	}
	if req.ProteinG != nil {
		v := float64(*req.ProteinG)
		d.ProteinG = &v
	}
	if req.FatG != nil {
		v := float64(*req.FatG)
		d.FatG = &v
	}
	if req.CarbsG != nil {
		v := float64(*req.CarbsG)
		d.CarbsG = &v
	}
	if req.Allergens != nil {
		d.Allergens = *req.Allergens
	}
	if req.Dietary != nil {
		d.Dietary = *req.Dietary
	}
	if req.TagIds != nil {
		d.TagIDs = *req.TagIds
	}
	return d
}

// PatchDishRequestToUsecase маппит api.PatchDishRequest в usecase.DishPatch
func PatchDishRequestToUsecase(req PatchDishRequest) usecasemodels.DishPatch {
	p := usecasemodels.DishPatch{
		Name:           req.Name,
		Description:    req.Description,
		Composition:    req.Composition,
		ImageURL:       req.ImageUrl,
		PriceMinor:     req.PriceMinor,
		Currency:       req.Currency,
		CategoryID:     req.CategoryId,
		CaloriesKcal:   req.CaloriesKcal,
		PortionWeightG: req.PortionWeightG,
		Allergens:      req.Allergens,
		Dietary:        req.Dietary,
		TagIDs:         req.TagIds,
		IsAvailable:    req.IsAvailable,
	}
	if req.Cuisine != nil {
		c := usecasemodels.Cuisine(*req.Cuisine)
		p.Cuisine = &c
	}
	if req.ProteinG != nil {
		v := float64(*req.ProteinG)
		p.ProteinG = &v
	}
	if req.FatG != nil {
		v := float64(*req.FatG)
		p.FatG = &v
	}
	if req.CarbsG != nil {
		v := float64(*req.CarbsG)
		p.CarbsG = &v
	}
	return p
}

// ListDishesParamsToUsecase маппит generated query-params в usecase.DishFilter
// Дефолты и верхняя граница limit должны быть применены до вызова (см. handler.fillListDishesDefaults)
func ListDishesParamsToUsecase(params ListDishesParams) usecasemodels.DishFilter {
	f := usecasemodels.DishFilter{
		CategoryID: params.CategoryId,
		Available:  params.Available,
	}
	if params.Q != nil {
		f.Q = *params.Q
	}
	if params.ExcludeAllergens != nil {
		f.ExcludeAllergens = *params.ExcludeAllergens
	}
	if params.Dietary != nil {
		f.Dietary = *params.Dietary
	}
	if params.TagIds != nil {
		f.TagIDs = *params.TagIds
	}
	if params.Limit != nil {
		f.Limit = *params.Limit
	}
	if params.Offset != nil {
		f.Offset = *params.Offset
	}
	return f
}

func tagsToAPI(ts []usecasemodels.Tag) []Tag {
	out := make([]Tag, len(ts))
	for i, t := range ts {
		out[i] = TagFromUsecase(t)
	}
	return out
}
