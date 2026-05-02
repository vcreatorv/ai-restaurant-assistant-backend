package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// newCartFixture возвращает Cart c фиксированным ID для удобства мока FindOrCreateCart.
func newCartFixture(userID uuid.UUID) *repositorymodels.Cart {
	return &repositorymodels.Cart{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// makeUsecase собирает usecase с моками
func makeUsecase(repo cart.Repository, m menu.Usecase) cart.Usecase {
	return New(Deps{Repo: repo, Menu: m})
}

// ---- AddItem ----

func TestAddItem_HappyPath(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	dish := &usecasemodels.Dish{ID: 7, Name: "Борщ", PriceMinor: 45000, IsAvailable: true}

	upsertCalled := false
	repo := &mockRepo{
		findOrCreateCartFn: func(_ context.Context, uid uuid.UUID) (*repositorymodels.Cart, error) {
			require.Equal(t, userID, uid)
			return cartRow, nil
		},
		findItemFn: func(_ context.Context, _ uuid.UUID, _ int) (*repositorymodels.CartItem, error) {
			return nil, cart.ErrCartItemNotFound
		},
		upsertItemFn: func(
			_ context.Context, cartID uuid.UUID, dishID, qty int, _ *string,
		) (*repositorymodels.CartItem, error) {
			upsertCalled = true
			require.Equal(t, cartRow.ID, cartID)
			require.Equal(t, 7, dishID)
			require.Equal(t, 2, qty)
			return &repositorymodels.CartItem{CartID: cartID, DishID: dishID, Quantity: 2, AddedAt: time.Now()}, nil
		},
		listItemsFn: func(_ context.Context, _ uuid.UUID) ([]repositorymodels.CartItem, error) {
			return []repositorymodels.CartItem{{CartID: cartRow.ID, DishID: 7, Quantity: 2, AddedAt: time.Now()}}, nil
		},
	}
	m := &mockMenu{
		getDishFn: func(_ context.Context, id int) (*usecasemodels.Dish, error) {
			require.Equal(t, 7, id)
			return dish, nil
		},
		getDishesByIDsFn: func(_ context.Context, _ []int) ([]usecasemodels.Dish, error) {
			return []usecasemodels.Dish{*dish}, nil
		},
	}

	view, err := makeUsecase(repo, m).AddItem(
		context.Background(), userID,
		usecasemodels.CartItemAdd{DishID: 7, Quantity: 2},
	)
	require.NoError(t, err)
	require.True(t, upsertCalled)
	require.Len(t, view.Items, 1)
	require.Equal(t, 7, view.Items[0].DishID)
	require.Equal(t, 90000, view.TotalMinor) // 45000 * 2
	require.Empty(t, view.Warnings)
}

func TestAddItem_QuantityBelowMin(t *testing.T) {
	uc := makeUsecase(&mockRepo{}, &mockMenu{})
	_, err := uc.AddItem(context.Background(), uuid.New(), usecasemodels.CartItemAdd{DishID: 1, Quantity: 0})
	require.ErrorIs(t, err, cart.ErrInvalidQuantity)
}

func TestAddItem_QuantityAboveMax(t *testing.T) {
	uc := makeUsecase(&mockRepo{}, &mockMenu{})
	_, err := uc.AddItem(
		context.Background(), uuid.New(),
		usecasemodels.CartItemAdd{DishID: 1, Quantity: cart.QuantityMax + 1},
	)
	require.ErrorIs(t, err, cart.ErrInvalidQuantity)
}

func TestAddItem_DishNotFound(t *testing.T) {
	m := &mockMenu{
		getDishFn: func(_ context.Context, _ int) (*usecasemodels.Dish, error) {
			return nil, menu.ErrDishNotFound
		},
	}
	_, err := makeUsecase(&mockRepo{}, m).AddItem(
		context.Background(), uuid.New(),
		usecasemodels.CartItemAdd{DishID: 999, Quantity: 1},
	)
	require.ErrorIs(t, err, cart.ErrDishNotFound)
}

func TestAddItem_DishUnavailable(t *testing.T) {
	m := &mockMenu{
		getDishFn: func(_ context.Context, _ int) (*usecasemodels.Dish, error) {
			return &usecasemodels.Dish{ID: 1, IsAvailable: false}, nil
		},
	}
	_, err := makeUsecase(&mockRepo{}, m).AddItem(
		context.Background(), uuid.New(),
		usecasemodels.CartItemAdd{DishID: 1, Quantity: 1},
	)
	require.ErrorIs(t, err, cart.ErrDishUnavailable)
}

func TestAddItem_SumExceedsMax(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		findItemFn: func(context.Context, uuid.UUID, int) (*repositorymodels.CartItem, error) {
			return &repositorymodels.CartItem{CartID: cartRow.ID, DishID: 1, Quantity: 49}, nil
		},
	}
	m := &mockMenu{
		getDishFn: func(context.Context, int) (*usecasemodels.Dish, error) {
			return &usecasemodels.Dish{ID: 1, IsAvailable: true, PriceMinor: 100}, nil
		},
	}
	// 49 + 2 > 50
	_, err := makeUsecase(repo, m).AddItem(context.Background(), userID, usecasemodels.CartItemAdd{DishID: 1, Quantity: 2})
	require.ErrorIs(t, err, cart.ErrInvalidQuantity)
}

func TestAddItem_UpsertError(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		findItemFn: func(context.Context, uuid.UUID, int) (*repositorymodels.CartItem, error) {
			return nil, cart.ErrCartItemNotFound
		},
		upsertItemFn: func(context.Context, uuid.UUID, int, int, *string) (*repositorymodels.CartItem, error) {
			return nil, errors.New("db fail")
		},
	}
	m := &mockMenu{
		getDishFn: func(context.Context, int) (*usecasemodels.Dish, error) {
			return &usecasemodels.Dish{ID: 1, IsAvailable: true}, nil
		},
	}
	_, err := makeUsecase(repo, m).AddItem(context.Background(), userID, usecasemodels.CartItemAdd{DishID: 1, Quantity: 1})
	require.Error(t, err)
}

// ---- Get / buildView ----

func TestGet_EmptyCart(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		listItemsFn: func(context.Context, uuid.UUID) ([]repositorymodels.CartItem, error) {
			return nil, nil
		},
	}
	view, err := makeUsecase(repo, &mockMenu{}).Get(context.Background(), userID)
	require.NoError(t, err)
	require.Empty(t, view.Items)
	require.Equal(t, "RUB", view.Currency)
	require.Equal(t, 0, view.TotalMinor)
}

func TestGet_WarningOnUnavailable(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		listItemsFn: func(context.Context, uuid.UUID) ([]repositorymodels.CartItem, error) {
			return []repositorymodels.CartItem{
				{CartID: cartRow.ID, DishID: 1, Quantity: 2},
				{CartID: cartRow.ID, DishID: 2, Quantity: 1},
			}, nil
		},
	}
	m := &mockMenu{
		getDishesByIDsFn: func(context.Context, []int) ([]usecasemodels.Dish, error) {
			return []usecasemodels.Dish{
				{ID: 1, Name: "A", PriceMinor: 100, IsAvailable: true},
				{ID: 2, Name: "B", PriceMinor: 500, IsAvailable: false}, // в стоп-листе
			}, nil
		},
	}
	view, err := makeUsecase(repo, m).Get(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, view.Items, 2)
	require.Len(t, view.Warnings, 1)
	require.Equal(t, 2, view.Warnings[0].DishID)
	require.Equal(t, usecasemodels.CartWarningDishUnavailable, view.Warnings[0].Code)
	// Total считается только по доступным позициям: 100 * 2 = 200
	require.Equal(t, 200, view.TotalMinor)
}

// ---- PatchItem ----

func TestPatchItem_QuantityValidation(t *testing.T) {
	uc := makeUsecase(&mockRepo{}, &mockMenu{})
	_, err := uc.PatchItem(context.Background(), uuid.New(), 1, usecasemodels.CartItemPatch{Quantity: ptr(0)})
	require.ErrorIs(t, err, cart.ErrInvalidQuantity)
	_, err = uc.PatchItem(
		context.Background(), uuid.New(), 1,
		usecasemodels.CartItemPatch{Quantity: ptr(cart.QuantityMax + 1)},
	)
	require.ErrorIs(t, err, cart.ErrInvalidQuantity)
}

func TestPatchItem_SetsQuantityAndFields(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)

	setQtyCalled, patchFieldsCalled := false, false
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		setItemQuantityFn: func(_ context.Context, _ uuid.UUID, dishID, qty int) (*repositorymodels.CartItem, error) {
			setQtyCalled = true
			require.Equal(t, 5, dishID)
			require.Equal(t, 3, qty)
			return &repositorymodels.CartItem{DishID: dishID, Quantity: qty}, nil
		},
		patchItemFieldsFn: func(
			_ context.Context, _ uuid.UUID, dishID int, note *string, sortOrder *int,
		) (*repositorymodels.CartItem, error) {
			patchFieldsCalled = true
			require.Equal(t, 5, dishID)
			require.NotNil(t, note)
			require.Equal(t, "без лука", *note)
			require.Nil(t, sortOrder)
			return &repositorymodels.CartItem{DishID: dishID, Note: note}, nil
		},
		listItemsFn: func(context.Context, uuid.UUID) ([]repositorymodels.CartItem, error) { return nil, nil },
	}
	_, err := makeUsecase(repo, &mockMenu{}).PatchItem(context.Background(), userID, 5, usecasemodels.CartItemPatch{
		Quantity: ptr(3),
		Note:     ptr("без лука"),
	})
	require.NoError(t, err)
	require.True(t, setQtyCalled)
	require.True(t, patchFieldsCalled)
}

// ---- RemoveItem / Clear ----

func TestRemoveItem_NotFound(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		deleteItemFn: func(context.Context, uuid.UUID, int) error {
			return cart.ErrCartItemNotFound
		},
	}
	err := makeUsecase(repo, &mockMenu{}).RemoveItem(context.Background(), userID, 42)
	require.ErrorIs(t, err, cart.ErrCartItemNotFound)
}

func TestClear_Ok(t *testing.T) {
	userID := uuid.New()
	cartRow := newCartFixture(userID)
	called := false
	repo := &mockRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) { return cartRow, nil },
		deleteAllItemsFn: func(_ context.Context, cartID uuid.UUID) error {
			called = true
			require.Equal(t, cartRow.ID, cartID)
			return nil
		},
	}
	require.NoError(t, makeUsecase(repo, &mockMenu{}).Clear(context.Background(), userID))
	require.True(t, called)
}
