package usecase

import (
	"testing"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/stretchr/testify/require"
)

func TestCuisine_IsValid(t *testing.T) {
	valid := []Cuisine{
		CuisineRussian, CuisineItalian, CuisineJapanese,
		CuisineFrench, CuisineAsian, CuisineEuropean, CuisineAmerican,
	}
	for _, c := range valid {
		require.Truef(t, c.IsValid(), "%s must be valid", c)
	}
	require.False(t, Cuisine("").IsValid())
	require.False(t, Cuisine("georgian").IsValid())
}

func TestDishFromRepository_Borsh(t *testing.T) {
	r := &repositorymodels.Dish{
		ID:             1,
		Name:           "Борщ с говядиной",
		Description:    "Классический борщ",
		Composition:    "Говядина, свёкла, капуста, картофель, морковь, лук, сметана",
		ImageURL:       "http://localhost:9000/restaurant/dishes/1.jpg",
		PriceMinor:     45000,
		Currency:       "RUB",
		CaloriesKcal:   ptr(320),
		ProteinG:       ptr(18.5),
		FatG:           ptr(15.0),
		CarbsG:         ptr(22.0),
		PortionWeightG: ptr(350),
		Cuisine:        "russian",
		CategoryID:     1,
		Allergens:      []string{"dairy"},
		Dietary:        []string{},
		IsAvailable:    true,
		Tags: []repositorymodels.Tag{
			{ID: 3, Name: "Хит сезона", Slug: "hit", Color: "#FB8C00"},
		},
	}
	d := DishFromRepository(r)

	require.Equal(t, 1, d.ID)
	require.Equal(t, "Борщ с говядиной", d.Name)
	require.Equal(t, 45000, d.PriceMinor)
	require.Equal(t, CuisineRussian, d.Cuisine)
	require.Equal(t, []string{"dairy"}, d.Allergens)
	require.Equal(t, []string{}, d.Dietary, "nil → пустой слайс")
	require.Len(t, d.Tags, 1)
	require.Equal(t, "hit", d.Tags[0].Slug)
}

func TestDishFromRepository_NullableNutrition(t *testing.T) {
	r := &repositorymodels.Dish{
		ID:             7,
		Name:           "Эспрессо",
		PriceMinor:     15000,
		Cuisine:        "italian",
		CategoryID:     11,
		IsAvailable:    true,
		CaloriesKcal:   nil,
		ProteinG:       nil,
		FatG:           nil,
		CarbsG:         nil,
		PortionWeightG: nil,
	}
	d := DishFromRepository(r)

	require.Nil(t, d.CaloriesKcal)
	require.Nil(t, d.ProteinG)
	require.Nil(t, d.PortionWeightG)
}

func TestDishFromRepository_Nil(t *testing.T) {
	require.Nil(t, DishFromRepository(nil))
}

func TestDishesFromRepository_PreservesOrder(t *testing.T) {
	rs := []repositorymodels.Dish{
		{ID: 1, Name: "A", PriceMinor: 100, Currency: "RUB", Cuisine: "european", CategoryID: 1, IsAvailable: true},
		{ID: 2, Name: "B", PriceMinor: 200, Currency: "RUB", Cuisine: "italian", CategoryID: 2, IsAvailable: false},
	}
	out := DishesFromRepository(rs)
	require.Len(t, out, 2)
	require.Equal(t, "A", out[0].Name)
	require.Equal(t, CuisineItalian, out[1].Cuisine)
	require.False(t, out[1].IsAvailable)
}

func TestDishFilterToRepository_PassesAllFields(t *testing.T) {
	available := true
	cat := 4
	f := DishFilter{
		CategoryID:       &cat,
		Available:        &available,
		Q:                "карбонар",
		ExcludeAllergens: []string{"dairy", "gluten"},
		Dietary:          []string{"halal"},
		TagIDs:           []int{3, 5},
		Limit:            20,
		Offset:           40,
	}
	r := DishFilterToRepository(f)

	require.Equal(t, &cat, r.CategoryID)
	require.Equal(t, []string{"dairy", "gluten"}, r.ExcludeAllergens)
	require.Equal(t, []int{3, 5}, r.TagIDs)
	require.Equal(t, 40, r.Offset)
}
