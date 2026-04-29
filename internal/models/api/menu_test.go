package api

import (
	"testing"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/stretchr/testify/require"
)

func TestDishFromUsecase_BorshWithTags(t *testing.T) {
	d := &usecasemodels.Dish{
		ID:             1,
		CategoryID:     1,
		Name:           "Борщ с говядиной",
		Description:    "Классический борщ",
		Composition:    "Говядина, свёкла, капуста, сметана",
		ImageURL:       "http://localhost:9000/restaurant/dishes/1.jpg",
		PriceMinor:     45000,
		Currency:       "RUB",
		CaloriesKcal:   ptr(320),
		ProteinG:       ptr(18.5),
		FatG:           ptr(15.0),
		CarbsG:         ptr(22.0),
		PortionWeightG: ptr(350),
		Cuisine:        usecasemodels.CuisineRussian,
		Allergens:      []string{"dairy"},
		Dietary:        []string{},
		IsAvailable:    true,
		Tags:           []usecasemodels.Tag{{ID: 3, Name: "Хит сезона", Slug: "hit", Color: "#FB8C00"}},
	}
	out := DishFromUsecase(d)

	require.Equal(t, 1, out.Id)
	require.Equal(t, 45000, out.PriceMinor)
	require.Equal(t, Cuisine("russian"), out.Cuisine)
	require.NotNil(t, out.ProteinG)
	require.InDelta(t, 18.5, float64(*out.ProteinG), 0.01)
	require.Len(t, out.Tags, 1)
	require.Equal(t, "hit", out.Tags[0].Slug)
}

func TestDishFromUsecase_NullableNutritionPointersStayNil(t *testing.T) {
	d := &usecasemodels.Dish{
		ID:          7,
		Name:        "Эспрессо",
		PriceMinor:  15000,
		Cuisine:     usecasemodels.CuisineItalian,
		IsAvailable: true,
	}
	out := DishFromUsecase(d)

	require.Nil(t, out.CaloriesKcal)
	require.Nil(t, out.ProteinG)
	require.Nil(t, out.PortionWeightG)
	require.Equal(t, []string{}, out.Allergens)
}

func TestDishListFromUsecase_PassesPagination(t *testing.T) {
	d := []usecasemodels.Dish{{
		ID: 1, Name: "x", PriceMinor: 100,
		Cuisine: usecasemodels.CuisineEuropean, IsAvailable: true,
	}}
	list := DishListFromUsecase(d, 42, 20, 0)
	require.Equal(t, 42, list.Total)
	require.Equal(t, 20, list.Limit)
	require.Equal(t, 0, list.Offset)
	require.Len(t, list.Items, 1)
}

func TestCreateDishRequestToUsecase_DefaultsAndOverrides(t *testing.T) {
	req := CreateDishRequest{
		Name:        "Карбонара",
		PriceMinor:  59000,
		Cuisine:     "italian",
		CategoryId:  4,
		Description: ptr("Спагетти с гуанчале"),
		Allergens:   ptr([]string{"gluten", "eggs", "dairy"}),
		TagIds:      ptr([]int{4, 6}),
	}
	d := CreateDishRequestToUsecase(req)

	require.Equal(t, "RUB", d.Currency, "currency по умолчанию")
	require.True(t, d.IsAvailable, "is_available по умолчанию true")
	require.Equal(t, "Спагетти с гуанчале", d.Description)
	require.Equal(t, []string{"gluten", "eggs", "dairy"}, d.Allergens)
	require.Equal(t, []int{4, 6}, d.TagIDs)
}

func TestCreateDishRequestToUsecase_ExplicitOverrides(t *testing.T) {
	req := CreateDishRequest{
		Name:        "Кофе",
		PriceMinor:  20000,
		Cuisine:     "italian",
		CategoryId:  11,
		Currency:    ptr("EUR"),
		IsAvailable: ptr(false),
	}
	d := CreateDishRequestToUsecase(req)

	require.Equal(t, "EUR", d.Currency)
	require.False(t, d.IsAvailable)
}

func TestPatchDishRequestToUsecase_PartialUpdate(t *testing.T) {
	req := PatchDishRequest{
		PriceMinor:  ptr(49000),
		Description: ptr("обновлено"),
	}
	p := PatchDishRequestToUsecase(req)

	require.NotNil(t, p.PriceMinor)
	require.Equal(t, 49000, *p.PriceMinor)
	require.NotNil(t, p.Description)
	require.Nil(t, p.Cuisine, "поля, не пришедшие в patch, остаются nil")
	require.Nil(t, p.IsAvailable)
}

func TestListDishesParamsToUsecase_AllFields(t *testing.T) {
	avail := true
	q := "карбонар"
	limit := 25
	offset := 10
	params := ListDishesParams{
		CategoryId:       ptr(4),
		Available:        &avail,
		Q:                &q,
		ExcludeAllergens: ptr([]string{"dairy"}),
		Dietary:          ptr([]string{"vegan"}),
		TagIds:           ptr([]int{3}),
		Limit:            &limit,
		Offset:           &offset,
	}
	f := ListDishesParamsToUsecase(params)

	require.Equal(t, 4, *f.CategoryID)
	require.Equal(t, "карбонар", f.Q)
	require.Equal(t, []string{"dairy"}, f.ExcludeAllergens)
	require.Equal(t, []int{3}, f.TagIDs)
	require.Equal(t, 25, f.Limit)
	require.Equal(t, 10, f.Offset)
}

func TestListDishesParamsToUsecase_EmptyParams(t *testing.T) {
	f := ListDishesParamsToUsecase(ListDishesParams{})
	require.Nil(t, f.CategoryID)
	require.Equal(t, 0, f.Limit, "дефолты накладываются handler.fillListDishesDefaults, конвертер их не выставляет")
	require.Equal(t, 0, f.Offset)
}
