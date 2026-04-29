package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// ----- categories -----

func TestCreateCategory_HappyPath(t *testing.T) {
	repo := &mockRepo{
		createCategoryFn: func(_ context.Context, c *repositorymodels.Category) error {
			c.ID = 42 // имитируем RETURNING id
			return nil
		},
	}
	uc := New(repo, nil)

	got, err := uc.CreateCategory(context.Background(), usecasemodels.CategoryCreate{
		Name:        "Супы",
		SortOrder:   30,
		IsAvailable: true,
	})
	require.NoError(t, err)
	require.Equal(t, 42, got.ID)
	require.Equal(t, "Супы", got.Name)
}

func TestCreateCategory_NameTaken(t *testing.T) {
	repo := &mockRepo{
		createCategoryFn: func(context.Context, *repositorymodels.Category) error {
			return menu.ErrCategoryNameTaken
		},
	}
	uc := New(repo, nil)

	_, err := uc.CreateCategory(context.Background(), usecasemodels.CategoryCreate{Name: "Супы"})
	require.ErrorIs(t, err, menu.ErrCategoryNameTaken)
}

func TestUpdateCategory_PartialPatch(t *testing.T) {
	stored := &repositorymodels.Category{ID: 1, Name: "Супы", SortOrder: 30, IsAvailable: true}
	repo := &mockRepo{
		findCategoryByIDFn: func(_ context.Context, id int) (*repositorymodels.Category, error) {
			require.Equal(t, 1, id)
			return stored, nil
		},
		updateCategoryFn: func(_ context.Context, c *repositorymodels.Category) error {
			require.Equal(t, "Супы", c.Name, "name не задан в патче — должен сохраниться")
			require.Equal(t, 5, c.SortOrder, "новый sort_order применился")
			require.False(t, c.IsAvailable)
			return nil
		},
	}
	uc := New(repo, nil)

	out, err := uc.UpdateCategory(context.Background(), 1, usecasemodels.CategoryPatch{
		SortOrder:   ptr(5),
		IsAvailable: ptr(false),
	})
	require.NoError(t, err)
	require.Equal(t, 5, out.SortOrder)
	require.False(t, out.IsAvailable)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	repo := &mockRepo{
		findCategoryByIDFn: func(context.Context, int) (*repositorymodels.Category, error) {
			return nil, menu.ErrCategoryNotFound
		},
	}
	uc := New(repo, nil)

	_, err := uc.UpdateCategory(context.Background(), 999, usecasemodels.CategoryPatch{Name: ptr("X")})
	require.ErrorIs(t, err, menu.ErrCategoryNotFound)
}

func TestDeleteCategory_BlockedByDishes(t *testing.T) {
	repo := &mockRepo{
		deleteCategoryFn: func(context.Context, int) error { return menu.ErrCategoryHasDishes },
	}
	uc := New(repo, nil)

	err := uc.DeleteCategory(context.Background(), 1)
	require.ErrorIs(t, err, menu.ErrCategoryHasDishes)
}

// ----- dishes -----

func TestCreateDish_HappyPathItalianPasta(t *testing.T) {
	repo := &mockRepo{
		createDishFn: func(_ context.Context, d *repositorymodels.Dish, tagIDs []int) error {
			d.ID = 7
			d.Tags = []repositorymodels.Tag{{ID: 4, Name: "Хит сезона", Slug: "hit"}}
			require.Equal(t, []int{4}, tagIDs, "tag_ids передаются в репо")
			require.Equal(t, "italian", d.Cuisine)
			return nil
		},
	}
	uc := New(repo, nil)

	got, err := uc.CreateDish(context.Background(), usecasemodels.DishCreate{
		Name:        "Карбонара",
		Description: "Спагетти с гуанчале",
		Composition: "Спагетти, гуанчале, желток, пекорино",
		PriceMinor:  59000,
		Cuisine:     usecasemodels.CuisineItalian,
		CategoryID:  4,
		Allergens:   []string{"gluten", "eggs", "dairy"},
		TagIDs:      []int{4},
		IsAvailable: true,
	})
	require.NoError(t, err)
	require.Equal(t, 7, got.ID)
	require.Equal(t, []string{"gluten", "eggs", "dairy"}, got.Allergens)
}

func TestCreateDish_InvalidCuisine(t *testing.T) {
	repo := &mockRepo{} // не должны попасть в repo
	uc := New(repo, nil)

	_, err := uc.CreateDish(context.Background(), usecasemodels.DishCreate{
		Name:    "Хинкали",
		Cuisine: "georgian", // нет в enum
	})
	require.ErrorIs(t, err, menu.ErrInvalidCuisine)
}

func TestCreateDish_DefaultCurrency(t *testing.T) {
	var observed string
	repo := &mockRepo{
		createDishFn: func(_ context.Context, d *repositorymodels.Dish, _ []int) error {
			observed = d.Currency
			d.ID = 1
			return nil
		},
	}
	uc := New(repo, nil)

	_, err := uc.CreateDish(context.Background(), usecasemodels.DishCreate{
		Name:       "Эспрессо",
		PriceMinor: 15000,
		Cuisine:    usecasemodels.CuisineItalian,
		CategoryID: 11,
	})
	require.NoError(t, err)
	require.Equal(t, "RUB", observed, "currency пустой → подставляется RUB")
}

func TestUpdateDish_PriceOnly_PreservesRest(t *testing.T) {
	stored := &repositorymodels.Dish{
		ID: 5, Name: "Карбонара", Description: "Спагетти", PriceMinor: 49000,
		Currency: "RUB", Cuisine: "italian", CategoryID: 4, IsAvailable: true,
		Allergens: []string{"gluten", "eggs", "dairy"},
	}
	repo := &mockRepo{
		findDishByIDFn: func(context.Context, int) (*repositorymodels.Dish, error) {
			return stored, nil // отражает актуальное состояние «БД»
		},
		updateDishFn: func(_ context.Context, d *repositorymodels.Dish, tagIDs []int) error {
			require.Equal(t, "Карбонара", d.Name)
			require.Equal(t, 59000, d.PriceMinor)
			require.Equal(t, []string{"gluten", "eggs", "dairy"}, d.Allergens, "не трогаем")
			require.Nil(t, tagIDs, "tag_ids не передан в патче — теги не перепривязываем")
			stored.PriceMinor = d.PriceMinor // имитируем коммит в БД
			return nil
		},
	}
	uc := New(repo, nil)

	out, err := uc.UpdateDish(context.Background(), 5, usecasemodels.DishPatch{
		PriceMinor: ptr(59000),
	})
	require.NoError(t, err)
	require.Equal(t, 59000, out.PriceMinor)
}

func TestUpdateDish_RetagsWhenTagIDsProvided(t *testing.T) {
	stored := &repositorymodels.Dish{
		ID: 5, Name: "Карбонара", PriceMinor: 49000, Cuisine: "italian", CategoryID: 4, IsAvailable: true,
	}
	repo := &mockRepo{
		findDishByIDFn: func(context.Context, int) (*repositorymodels.Dish, error) { return stored, nil },
		updateDishFn: func(_ context.Context, _ *repositorymodels.Dish, tagIDs []int) error {
			require.Equal(t, []int{1, 2}, tagIDs)
			return nil
		},
	}
	uc := New(repo, nil)

	_, err := uc.UpdateDish(context.Background(), 5, usecasemodels.DishPatch{TagIDs: ptr([]int{1, 2})})
	require.NoError(t, err)
}

func TestUpdateDish_NotFound(t *testing.T) {
	repo := &mockRepo{
		findDishByIDFn: func(context.Context, int) (*repositorymodels.Dish, error) {
			return nil, menu.ErrDishNotFound
		},
	}
	uc := New(repo, nil)
	_, err := uc.UpdateDish(context.Background(), 999, usecasemodels.DishPatch{Name: ptr("x")})
	require.ErrorIs(t, err, menu.ErrDishNotFound)
}

func TestDeleteDish_SoftDeleteCalls(t *testing.T) {
	var capturedID int
	var capturedAvail bool
	repo := &mockRepo{
		setDishAvailabilityFn: func(_ context.Context, id int, avail bool) error {
			capturedID, capturedAvail = id, avail
			return nil
		},
	}
	uc := New(repo, nil)

	require.NoError(t, uc.DeleteDish(context.Background(), 5))
	require.Equal(t, 5, capturedID)
	require.False(t, capturedAvail, "удаление = SetAvailability(false)")
}

// ----- upload image -----

func TestUploadDishImage_HappyPath(t *testing.T) {
	stored := &repositorymodels.Dish{
		ID: 5, Name: "Карбонара", PriceMinor: 49000, Cuisine: "italian", CategoryID: 4, IsAvailable: true,
	}
	updated := &repositorymodels.Dish{
		ID: 5, Name: "Карбонара", PriceMinor: 49000, Cuisine: "italian", CategoryID: 4, IsAvailable: true,
		ImageURL: "http://localhost:9000/restaurant/dishes/5-123.jpg",
	}
	repo := &mockRepo{
		findDishByIDFn: func(_ context.Context, id int) (*repositorymodels.Dish, error) {
			require.Equal(t, 5, id)
			if updated.ImageURL != "" {
				return updated, nil
			}
			return stored, nil
		},
		updateDishFn: func(_ context.Context, d *repositorymodels.Dish, tagIDs []int) error {
			require.Nil(t, tagIDs, "загрузка картинки не должна трогать теги")
			require.NotEmpty(t, d.ImageURL)
			updated.ImageURL = d.ImageURL
			return nil
		},
	}
	storage := &mockStorage{
		uploadFn: func(_ context.Context, key, ct string, body io.Reader, size int64) (string, error) {
			require.True(t, strings.HasPrefix(key, "dishes/5-"), "ключ начинается с dishes/<id>-")
			require.Equal(t, "image/jpeg", ct)
			require.Equal(t, int64(3), size)
			return "http://localhost:9000/restaurant/" + key, nil
		},
	}
	uc := New(repo, storage)

	out, err := uc.UploadDishImage(context.Background(), 5, menu.DishImageSource{
		Body: bytes.NewReader([]byte{0xff, 0xd8, 0xff}), ContentType: "image/jpeg", Size: 3, Ext: "jpg",
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.ImageURL)
}

func TestUploadDishImage_DishNotFound_NoUpload(t *testing.T) {
	storageCalled := false
	storage := &mockStorage{
		uploadFn: func(context.Context, string, string, io.Reader, int64) (string, error) {
			storageCalled = true
			return "", nil
		},
	}
	repo := &mockRepo{
		findDishByIDFn: func(context.Context, int) (*repositorymodels.Dish, error) {
			return nil, menu.ErrDishNotFound
		},
	}
	uc := New(repo, storage)

	_, err := uc.UploadDishImage(context.Background(), 999, menu.DishImageSource{
		Body: bytes.NewReader(nil), ContentType: "image/jpeg", Ext: "jpg",
	})
	require.ErrorIs(t, err, menu.ErrDishNotFound)
	require.False(t, storageCalled, "если блюда нет, в S3 не лезем")
}

func TestUploadDishImage_StorageError_PropagatesNoUpdate(t *testing.T) {
	stored := &repositorymodels.Dish{
		ID: 5, Name: "x", PriceMinor: 100,
		Cuisine: "italian", CategoryID: 1, IsAvailable: true,
	}
	updateCalled := false
	repo := &mockRepo{
		findDishByIDFn: func(context.Context, int) (*repositorymodels.Dish, error) { return stored, nil },
		updateDishFn: func(context.Context, *repositorymodels.Dish, []int) error {
			updateCalled = true
			return nil
		},
	}
	storage := &mockStorage{
		uploadFn: func(context.Context, string, string, io.Reader, int64) (string, error) {
			return "", errors.New("s3 connection refused")
		},
	}
	uc := New(repo, storage)

	_, err := uc.UploadDishImage(context.Background(), 5, menu.DishImageSource{
		Body: bytes.NewReader(nil), ContentType: "image/jpeg", Ext: "jpg",
	})
	require.Error(t, err)
	require.False(t, updateCalled, "при провале upload не должны менять image_url в БД")
}
