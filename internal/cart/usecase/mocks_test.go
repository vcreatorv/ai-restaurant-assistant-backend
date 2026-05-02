package usecase

import (
	"context"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	"github.com/google/uuid"
)

// mockRepo ручная реализация cart.Repository.
// Не выставленные fn-поля паникуют при вызове — это сигнал, что тест задел метод,
// который не должен.
type mockRepo struct {
	findOrCreateCartFn func(ctx context.Context, userID uuid.UUID) (*repositorymodels.Cart, error)
	listItemsFn        func(ctx context.Context, cartID uuid.UUID) ([]repositorymodels.CartItem, error)
	upsertItemFn       func(ctx context.Context, cartID uuid.UUID, dishID, qtyDelta int, note *string) (*repositorymodels.CartItem, error)
	setItemQuantityFn  func(ctx context.Context, cartID uuid.UUID, dishID, quantity int) (*repositorymodels.CartItem, error)
	patchItemFieldsFn  func(ctx context.Context, cartID uuid.UUID, dishID int, note *string, sortOrder *int) (*repositorymodels.CartItem, error)
	findItemFn         func(ctx context.Context, cartID uuid.UUID, dishID int) (*repositorymodels.CartItem, error)
	deleteItemFn       func(ctx context.Context, cartID uuid.UUID, dishID int) error
	deleteAllItemsFn   func(ctx context.Context, cartID uuid.UUID) error
}

func (m *mockRepo) FindOrCreateCart(ctx context.Context, userID uuid.UUID) (*repositorymodels.Cart, error) {
	return m.findOrCreateCartFn(ctx, userID)
}
func (m *mockRepo) ListItems(ctx context.Context, cartID uuid.UUID) ([]repositorymodels.CartItem, error) {
	return m.listItemsFn(ctx, cartID)
}
func (m *mockRepo) UpsertItem(ctx context.Context, cartID uuid.UUID, dishID, quantityDelta int, note *string) (*repositorymodels.CartItem, error) {
	return m.upsertItemFn(ctx, cartID, dishID, quantityDelta, note)
}
func (m *mockRepo) SetItemQuantity(ctx context.Context, cartID uuid.UUID, dishID, quantity int) (*repositorymodels.CartItem, error) {
	return m.setItemQuantityFn(ctx, cartID, dishID, quantity)
}
func (m *mockRepo) PatchItemFields(ctx context.Context, cartID uuid.UUID, dishID int, note *string, sortOrder *int) (*repositorymodels.CartItem, error) {
	return m.patchItemFieldsFn(ctx, cartID, dishID, note, sortOrder)
}
func (m *mockRepo) FindItem(ctx context.Context, cartID uuid.UUID, dishID int) (*repositorymodels.CartItem, error) {
	return m.findItemFn(ctx, cartID, dishID)
}
func (m *mockRepo) DeleteItem(ctx context.Context, cartID uuid.UUID, dishID int) error {
	return m.deleteItemFn(ctx, cartID, dishID)
}
func (m *mockRepo) DeleteAllItems(ctx context.Context, cartID uuid.UUID) error {
	return m.deleteAllItemsFn(ctx, cartID)
}

// mockMenu ручная реализация menu.Usecase. Используются только GetDish и GetDishesByIDs;
// остальные методы паникуют — cart.Usecase их не должен дёргать.
type mockMenu struct {
	getDishFn        func(ctx context.Context, id int) (*usecasemodels.Dish, error)
	getDishesByIDsFn func(ctx context.Context, ids []int) ([]usecasemodels.Dish, error)
}

func (m *mockMenu) GetDish(ctx context.Context, id int) (*usecasemodels.Dish, error) {
	return m.getDishFn(ctx, id)
}
func (m *mockMenu) GetDishesByIDs(ctx context.Context, ids []int) ([]usecasemodels.Dish, error) {
	return m.getDishesByIDsFn(ctx, ids)
}

// --- остальные методы menu.Usecase: stub'ы, которые тесты cart не вызывают ---

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
