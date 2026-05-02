package usecase

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"
)

// mockOrderRepo ручная реализация order.Repository
type mockOrderRepo struct {
	createOrderFn       func(ctx context.Context, o *repositorymodels.Order, items []repositorymodels.OrderItem) error
	findOrderByIDFn     func(ctx context.Context, id uuid.UUID) (*repositorymodels.Order, []repositorymodels.OrderItem, error)
	listOrdersFn        func(ctx context.Context, f repositorymodels.OrderFilter) ([]repositorymodels.Order, int, error)
	loadOrderItemsFn    func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]repositorymodels.OrderItem, error)
	updateOrderStatusFn func(ctx context.Context, id uuid.UUID, status string) (*repositorymodels.Order, error)
}

func (m *mockOrderRepo) CreateOrder(ctx context.Context, o *repositorymodels.Order, items []repositorymodels.OrderItem) error {
	return m.createOrderFn(ctx, o, items)
}
func (m *mockOrderRepo) FindOrderByID(ctx context.Context, id uuid.UUID) (*repositorymodels.Order, []repositorymodels.OrderItem, error) {
	return m.findOrderByIDFn(ctx, id)
}
func (m *mockOrderRepo) ListOrders(ctx context.Context, f repositorymodels.OrderFilter) ([]repositorymodels.Order, int, error) {
	return m.listOrdersFn(ctx, f)
}
func (m *mockOrderRepo) LoadOrderItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]repositorymodels.OrderItem, error) {
	return m.loadOrderItemsFn(ctx, ids)
}
func (m *mockOrderRepo) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*repositorymodels.Order, error) {
	return m.updateOrderStatusFn(ctx, id, status)
}

// mockCartRepo ручная реализация cart.Repository (из них order трогает FindOrCreateCart, ListItems, DeleteAllItems)
type mockCartRepo struct {
	findOrCreateCartFn func(ctx context.Context, userID uuid.UUID) (*repositorymodels.Cart, error)
	listItemsFn        func(ctx context.Context, cartID uuid.UUID) ([]repositorymodels.CartItem, error)
	deleteAllItemsFn   func(ctx context.Context, cartID uuid.UUID) error
}

func (m *mockCartRepo) FindOrCreateCart(ctx context.Context, userID uuid.UUID) (*repositorymodels.Cart, error) {
	return m.findOrCreateCartFn(ctx, userID)
}
func (m *mockCartRepo) ListItems(ctx context.Context, cartID uuid.UUID) ([]repositorymodels.CartItem, error) {
	return m.listItemsFn(ctx, cartID)
}
func (m *mockCartRepo) DeleteAllItems(ctx context.Context, cartID uuid.UUID) error {
	return m.deleteAllItemsFn(ctx, cartID)
}
func (*mockCartRepo) UpsertItem(context.Context, uuid.UUID, int, int, *string) (*repositorymodels.CartItem, error) {
	panic("UpsertItem not expected")
}
func (*mockCartRepo) SetItemQuantity(context.Context, uuid.UUID, int, int) (*repositorymodels.CartItem, error) {
	panic("SetItemQuantity not expected")
}
func (*mockCartRepo) PatchItemFields(context.Context, uuid.UUID, int, *string, *int) (*repositorymodels.CartItem, error) {
	panic("PatchItemFields not expected")
}
func (*mockCartRepo) FindItem(context.Context, uuid.UUID, int) (*repositorymodels.CartItem, error) {
	panic("FindItem not expected")
}
func (*mockCartRepo) DeleteItem(context.Context, uuid.UUID, int) error {
	panic("DeleteItem not expected")
}

// mockMenu ручная реализация menu.Usecase — order трогает только GetDishesByIDs
type mockMenu struct {
	getDishesByIDsFn func(ctx context.Context, ids []int) ([]usecasemodels.Dish, error)
}

func (m *mockMenu) GetDishesByIDs(ctx context.Context, ids []int) ([]usecasemodels.Dish, error) {
	return m.getDishesByIDsFn(ctx, ids)
}
func (*mockMenu) GetDish(context.Context, int) (*usecasemodels.Dish, error) {
	panic("GetDish not expected")
}
func (*mockMenu) ListCategories(context.Context) ([]usecasemodels.Category, error) {
	panic("ListCategories not expected")
}
func (*mockMenu) CreateCategory(context.Context, usecasemodels.CategoryCreate) (*usecasemodels.Category, error) {
	panic("CreateCategory not expected")
}
func (*mockMenu) UpdateCategory(context.Context, int, usecasemodels.CategoryPatch) (*usecasemodels.Category, error) {
	panic("UpdateCategory not expected")
}
func (*mockMenu) DeleteCategory(context.Context, int) error { panic("DeleteCategory not expected") }
func (*mockMenu) ListTags(context.Context) ([]usecasemodels.Tag, error) {
	panic("ListTags not expected")
}
func (*mockMenu) CreateTag(context.Context, usecasemodels.TagCreate) (*usecasemodels.Tag, error) {
	panic("CreateTag not expected")
}
func (*mockMenu) UpdateTag(context.Context, int, usecasemodels.TagPatch) (*usecasemodels.Tag, error) {
	panic("UpdateTag not expected")
}
func (*mockMenu) DeleteTag(context.Context, int) error { panic("DeleteTag not expected") }
func (*mockMenu) ListDishes(context.Context, usecasemodels.DishFilter) ([]usecasemodels.Dish, int, error) {
	panic("ListDishes not expected")
}
func (*mockMenu) CreateDish(context.Context, usecasemodels.DishCreate) (*usecasemodels.Dish, error) {
	panic("CreateDish not expected")
}
func (*mockMenu) UpdateDish(context.Context, int, usecasemodels.DishPatch) (*usecasemodels.Dish, error) {
	panic("UpdateDish not expected")
}
func (*mockMenu) DeleteDish(context.Context, int) error { panic("DeleteDish not expected") }
func (*mockMenu) UploadDishImage(context.Context, int, menu.DishImageSource) (*usecasemodels.Dish, error) {
	panic("UploadDishImage not expected")
}

// mockUsers ручная реализация user.Usecase — order трогает только GetByID
type mockUsers struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*usecasemodels.User, error)
}

func (m *mockUsers) GetByID(ctx context.Context, id uuid.UUID) (*usecasemodels.User, error) {
	return m.getByIDFn(ctx, id)
}
func (*mockUsers) GetProfile(context.Context, uuid.UUID) (*usecasemodels.User, error) {
	panic("GetProfile not expected")
}
func (*mockUsers) UpdateProfile(context.Context, uuid.UUID, usecasemodels.ProfilePatch) (*usecasemodels.User, error) {
	panic("UpdateProfile not expected")
}

// fixedUUID детерминированный UUIDGen
type fixedUUID struct{ id uuid.UUID }

func (f fixedUUID) New() uuid.UUID { return f.id }

var _ user.Usecase = (*mockUsers)(nil)
