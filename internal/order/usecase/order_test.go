package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/order"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func newOrderUsecase(d Deps) order.Usecase { return New(d) }

// usersWithProfile возвращает mockUsers, у которого GetByID отдаёт валидный профиль userID
func usersWithProfile(userID uuid.UUID) *mockUsers {
	return &mockUsers{
		getByIDFn: func(context.Context, uuid.UUID) (*usecasemodels.User, error) {
			return fullProfile(userID), nil
		},
	}
}

// fullProfile валидный профиль
func fullProfile(id uuid.UUID) *usecasemodels.User {
	return &usecasemodels.User{
		ID:        id,
		FirstName: "Иван",
		LastName:  "Иванов",
		Phone:     "+79991112233",
		Email:     "i@example.com",
	}
}

// ----- Create -----

func TestCreate_HappyPathPickup(t *testing.T) {
	userID := uuid.New()
	cartID := uuid.New()
	orderID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	repo := &mockOrderRepo{
		createOrderFn: func(_ context.Context, o *repositorymodels.Order, items []repositorymodels.OrderItem) error {
			require.Equal(t, orderID, o.ID)
			require.Equal(t, userID, o.UserID)
			require.Equal(t, "accepted", o.Status)
			require.Equal(t, "pickup", o.FulfillmentType)
			require.Equal(t, "on_delivery", o.PaymentMethod)
			// 100 * 2 + 250 * 1 = 450
			require.Equal(t, 450, o.TotalMinor)
			require.Equal(t, "RUB", o.Currency)
			require.Nil(t, o.DeliveryAddress, "не-delivery — адрес должен быть nil")
			require.Len(t, items, 2)
			// snapshot цен/имён должен быть из dish
			require.Equal(t, "Борщ", items[0].NameSnapshot)
			require.Equal(t, 100, items[0].PriceMinorSnapshot)
			return nil
		},
	}
	cartRepo := &mockCartRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) {
			return &repositorymodels.Cart{ID: cartID, UserID: userID}, nil
		},
		listItemsFn: func(_ context.Context, id uuid.UUID) ([]repositorymodels.CartItem, error) {
			require.Equal(t, cartID, id)
			return []repositorymodels.CartItem{
				{CartID: cartID, DishID: 1, Quantity: 2, AddedAt: time.Now()},
				{CartID: cartID, DishID: 2, Quantity: 1, AddedAt: time.Now()},
			}, nil
		},
		deleteAllItemsFn: func(_ context.Context, id uuid.UUID) error {
			require.Equal(t, cartID, id)
			return nil
		},
	}
	m := &mockMenu{
		getDishesByIDsFn: func(context.Context, []int) ([]usecasemodels.Dish, error) {
			return []usecasemodels.Dish{
				{ID: 1, Name: "Борщ", PriceMinor: 100, IsAvailable: true},
				{ID: 2, Name: "Уха", PriceMinor: 250, IsAvailable: true},
			}, nil
		},
	}
	users := &mockUsers{
		getByIDFn: func(context.Context, uuid.UUID) (*usecasemodels.User, error) {
			return fullProfile(userID), nil
		},
	}

	uc := newOrderUsecase(Deps{Repo: repo, CartRepo: cartRepo, Menu: m, Users: users, UUID: fixedUUID{id: orderID}})
	got, err := uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentPickup,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.NoError(t, err)
	require.Equal(t, orderID, got.ID)
	require.Equal(t, 450, got.TotalMinor)
	require.Len(t, got.Items, 2)
}

func TestCreate_DeliveryRequiresAddress(t *testing.T) {
	userID := uuid.New()
	users := usersWithProfile(userID)
	uc := newOrderUsecase(Deps{Users: users})
	_, err := uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentDelivery,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
		DeliveryAddress: ptr("   "),
	})
	require.ErrorIs(t, err, order.ErrDeliveryAddressRequired)

	_, err = uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentDelivery,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.ErrorIs(t, err, order.ErrDeliveryAddressRequired)
}

func TestCreate_InvalidFulfillment(t *testing.T) {
	uc := newOrderUsecase(Deps{})
	_, err := uc.Create(context.Background(), uuid.New(), usecasemodels.CreateOrderRequest{
		FulfillmentType: "wat",
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.ErrorIs(t, err, order.ErrInvalidFulfillmentType)
}

func TestCreate_InvalidPayment(t *testing.T) {
	uc := newOrderUsecase(Deps{})
	_, err := uc.Create(context.Background(), uuid.New(), usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentPickup,
		PaymentMethod:   "bitcoin",
	})
	require.ErrorIs(t, err, order.ErrInvalidPaymentMethod)
}

func TestCreate_ProfileIncomplete(t *testing.T) {
	userID := uuid.New()
	users := &mockUsers{getByIDFn: func(context.Context, uuid.UUID) (*usecasemodels.User, error) {
		return &usecasemodels.User{ID: userID, FirstName: "Иван"}, nil // нет фамилии и телефона
	}}
	uc := newOrderUsecase(Deps{Users: users})
	_, err := uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentPickup,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.ErrorIs(t, err, order.ErrProfileIncomplete)
}

func TestCreate_CartEmpty(t *testing.T) {
	userID := uuid.New()
	cartRepo := &mockCartRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) {
			return &repositorymodels.Cart{ID: uuid.New()}, nil
		},
		listItemsFn: func(context.Context, uuid.UUID) ([]repositorymodels.CartItem, error) { return nil, nil },
	}
	users := usersWithProfile(userID)
	uc := newOrderUsecase(Deps{CartRepo: cartRepo, Users: users})
	_, err := uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentPickup,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.ErrorIs(t, err, order.ErrCartEmpty)
}

func TestCreate_StopListItems(t *testing.T) {
	userID := uuid.New()
	cartID := uuid.New()
	cartRepo := &mockCartRepo{
		findOrCreateCartFn: func(context.Context, uuid.UUID) (*repositorymodels.Cart, error) {
			return &repositorymodels.Cart{ID: cartID}, nil
		},
		listItemsFn: func(context.Context, uuid.UUID) ([]repositorymodels.CartItem, error) {
			return []repositorymodels.CartItem{
				{CartID: cartID, DishID: 1, Quantity: 1},
				{CartID: cartID, DishID: 2, Quantity: 3},
			}, nil
		},
	}
	m := &mockMenu{
		getDishesByIDsFn: func(context.Context, []int) ([]usecasemodels.Dish, error) {
			return []usecasemodels.Dish{
				{ID: 1, IsAvailable: true, PriceMinor: 100, Name: "A"},
				{ID: 2, IsAvailable: false, PriceMinor: 200, Name: "B"},
			}, nil
		},
	}
	users := usersWithProfile(userID)
	uc := newOrderUsecase(Deps{CartRepo: cartRepo, Menu: m, Users: users})
	_, err := uc.Create(context.Background(), userID, usecasemodels.CreateOrderRequest{
		FulfillmentType: usecasemodels.FulfillmentPickup,
		PaymentMethod:   usecasemodels.PaymentOnDelivery,
	})
	require.ErrorIs(t, err, order.ErrCheckoutItemsUnavailable)
	var ce *order.CheckoutItemsUnavailableError
	require.ErrorAs(t, err, &ce)
	require.Equal(t, []int{2}, ce.DishIDs)
}

// ----- AdminUpdateStatus / state machine -----

func TestAdminUpdateStatus_AllAllowedTransitions(t *testing.T) {
	cases := []struct {
		from, to usecasemodels.OrderStatus
	}{
		{usecasemodels.OrderStatusAccepted, usecasemodels.OrderStatusCooking},
		{usecasemodels.OrderStatusAccepted, usecasemodels.OrderStatusCancelled},
		{usecasemodels.OrderStatusCooking, usecasemodels.OrderStatusReady},
		{usecasemodels.OrderStatusCooking, usecasemodels.OrderStatusCancelled},
		{usecasemodels.OrderStatusReady, usecasemodels.OrderStatusInDelivery},
		{usecasemodels.OrderStatusReady, usecasemodels.OrderStatusClosed},
		{usecasemodels.OrderStatusReady, usecasemodels.OrderStatusCancelled},
		{usecasemodels.OrderStatusInDelivery, usecasemodels.OrderStatusClosed},
		{usecasemodels.OrderStatusInDelivery, usecasemodels.OrderStatusCancelled},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			require.True(t, canTransition(tc.from, tc.to))
		})
	}
}

func TestAdminUpdateStatus_DisallowedTransitions(t *testing.T) {
	cases := []struct {
		from, to usecasemodels.OrderStatus
	}{
		{usecasemodels.OrderStatusClosed, usecasemodels.OrderStatusCooking},      // из терминала
		{usecasemodels.OrderStatusCancelled, usecasemodels.OrderStatusAccepted},  // из терминала
		{usecasemodels.OrderStatusAccepted, usecasemodels.OrderStatusReady},      // пропуск этапа
		{usecasemodels.OrderStatusAccepted, usecasemodels.OrderStatusInDelivery}, // пропуск этапа
		{usecasemodels.OrderStatusReady, usecasemodels.OrderStatusCooking},       // назад
		{usecasemodels.OrderStatusCooking, usecasemodels.OrderStatusAccepted},    // назад
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			require.False(t, canTransition(tc.from, tc.to))
		})
	}
}

func TestAdminUpdateStatus_InvalidStatus(t *testing.T) {
	uc := newOrderUsecase(Deps{})
	_, err := uc.AdminUpdateStatus(context.Background(), uuid.New(), "weird")
	require.ErrorIs(t, err, order.ErrInvalidStatus)
}

func TestAdminUpdateStatus_TransitionRejected(t *testing.T) {
	orderID := uuid.New()
	repo := &mockOrderRepo{
		findOrderByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
			return &repositorymodels.Order{ID: orderID, Status: "closed"}, nil, nil
		},
	}
	uc := newOrderUsecase(Deps{Repo: repo})
	_, err := uc.AdminUpdateStatus(context.Background(), orderID, usecasemodels.OrderStatusCooking)
	require.ErrorIs(t, err, order.ErrInvalidStatusTransition)
}

func TestAdminUpdateStatus_HappyPath(t *testing.T) {
	orderID := uuid.New()
	updateCalled := false
	repo := &mockOrderRepo{
		findOrderByIDFn: func(
			context.Context, uuid.UUID,
		) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
			return &repositorymodels.Order{
				ID: orderID, Status: "accepted",
				FulfillmentType: "pickup", PaymentMethod: "on_delivery",
			}, nil, nil
		},
		updateOrderStatusFn: func(
			_ context.Context, id uuid.UUID, status string,
		) (*repositorymodels.Order, error) {
			updateCalled = true
			require.Equal(t, orderID, id)
			require.Equal(t, "cooking", status)
			return &repositorymodels.Order{
				ID: orderID, Status: status,
				FulfillmentType: "pickup", PaymentMethod: "on_delivery",
			}, nil
		},
	}
	uc := newOrderUsecase(Deps{Repo: repo})
	got, err := uc.AdminUpdateStatus(context.Background(), orderID, usecasemodels.OrderStatusCooking)
	require.NoError(t, err)
	require.True(t, updateCalled)
	require.Equal(t, usecasemodels.OrderStatusCooking, got.Status)
}

func TestAdminUpdateStatus_OrderNotFound(t *testing.T) {
	repo := &mockOrderRepo{
		findOrderByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
			return nil, nil, order.ErrOrderNotFound
		},
	}
	uc := newOrderUsecase(Deps{Repo: repo})
	_, err := uc.AdminUpdateStatus(context.Background(), uuid.New(), usecasemodels.OrderStatusCooking)
	require.ErrorIs(t, err, order.ErrOrderNotFound)
}

// ----- Get / ownership -----

func TestGet_ForbiddenForOtherUser(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	orderID := uuid.New()
	repo := &mockOrderRepo{
		findOrderByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
			return &repositorymodels.Order{ID: orderID, UserID: owner, Status: "accepted"}, nil, nil
		},
	}
	uc := newOrderUsecase(Deps{Repo: repo})
	_, err := uc.Get(context.Background(), other, orderID)
	require.ErrorIs(t, err, order.ErrOrderForbidden)
}

// гарантия, что cart-интерфейс не сломается на этих моках
var _ cart.Repository = (*mockCartRepo)(nil)

// предохранитель: ошибки errors.Is должны корректно работать с CheckoutItemsUnavailableError
func TestCheckoutItemsUnavailableError_IsAndAs(t *testing.T) {
	src := &order.CheckoutItemsUnavailableError{DishIDs: []int{1, 2}}
	require.True(t, errors.Is(src, order.ErrCheckoutItemsUnavailable))
	var dst *order.CheckoutItemsUnavailableError
	require.True(t, errors.As(src, &dst))
	require.Equal(t, []int{1, 2}, dst.DishIDs)
}
