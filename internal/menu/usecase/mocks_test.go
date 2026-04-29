package usecase

import (
	"context"
	"io"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
)

// mockRepo минимальная ручная реализация menu.Repository для тестов.
// Поведение метода настраивается через func-поле; не выставленные поля паникуют при вызове —
// это сигнал, что тест задел метод, который не должен.
type mockRepo struct {
	listCategoriesFn      func(ctx context.Context) ([]repositorymodels.Category, error)
	findCategoryByIDFn    func(ctx context.Context, id int) (*repositorymodels.Category, error)
	createCategoryFn      func(ctx context.Context, c *repositorymodels.Category) error
	updateCategoryFn      func(ctx context.Context, c *repositorymodels.Category) error
	deleteCategoryFn      func(ctx context.Context, id int) error
	listTagsFn            func(ctx context.Context) ([]repositorymodels.Tag, error)
	findTagsByIDsFn       func(ctx context.Context, ids []int) ([]repositorymodels.Tag, error)
	findTagByIDFn         func(ctx context.Context, id int) (*repositorymodels.Tag, error)
	createTagFn           func(ctx context.Context, t *repositorymodels.Tag) error
	updateTagFn           func(ctx context.Context, t *repositorymodels.Tag) error
	deleteTagFn           func(ctx context.Context, id int) error
	listDishesFn          func(ctx context.Context, f repositorymodels.DishFilter) ([]repositorymodels.Dish, int, error)
	findDishByIDFn        func(ctx context.Context, id int) (*repositorymodels.Dish, error)
	createDishFn          func(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	updateDishFn          func(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	setDishAvailabilityFn func(ctx context.Context, id int, available bool) error
}

func (m *mockRepo) ListCategories(ctx context.Context) ([]repositorymodels.Category, error) {
	return m.listCategoriesFn(ctx)
}
func (m *mockRepo) FindCategoryByID(ctx context.Context, id int) (*repositorymodels.Category, error) {
	return m.findCategoryByIDFn(ctx, id)
}
func (m *mockRepo) CreateCategory(ctx context.Context, c *repositorymodels.Category) error {
	return m.createCategoryFn(ctx, c)
}
func (m *mockRepo) UpdateCategory(ctx context.Context, c *repositorymodels.Category) error {
	return m.updateCategoryFn(ctx, c)
}
func (m *mockRepo) DeleteCategory(ctx context.Context, id int) error {
	return m.deleteCategoryFn(ctx, id)
}
func (m *mockRepo) ListTags(ctx context.Context) ([]repositorymodels.Tag, error) {
	return m.listTagsFn(ctx)
}
func (m *mockRepo) FindTagsByIDs(ctx context.Context, ids []int) ([]repositorymodels.Tag, error) {
	return m.findTagsByIDsFn(ctx, ids)
}
func (m *mockRepo) FindTagByID(ctx context.Context, id int) (*repositorymodels.Tag, error) {
	return m.findTagByIDFn(ctx, id)
}
func (m *mockRepo) CreateTag(ctx context.Context, t *repositorymodels.Tag) error {
	return m.createTagFn(ctx, t)
}
func (m *mockRepo) UpdateTag(ctx context.Context, t *repositorymodels.Tag) error {
	return m.updateTagFn(ctx, t)
}
func (m *mockRepo) DeleteTag(ctx context.Context, id int) error {
	return m.deleteTagFn(ctx, id)
}
func (m *mockRepo) ListDishes(ctx context.Context, f repositorymodels.DishFilter) ([]repositorymodels.Dish, int, error) {
	return m.listDishesFn(ctx, f)
}
func (m *mockRepo) FindDishByID(ctx context.Context, id int) (*repositorymodels.Dish, error) {
	return m.findDishByIDFn(ctx, id)
}
func (m *mockRepo) CreateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error {
	return m.createDishFn(ctx, d, tagIDs)
}
func (m *mockRepo) UpdateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error {
	return m.updateDishFn(ctx, d, tagIDs)
}
func (m *mockRepo) SetDishAvailability(ctx context.Context, id int, available bool) error {
	return m.setDishAvailabilityFn(ctx, id, available)
}

// mockStorage ручная реализация s3.Storage
type mockStorage struct {
	uploadFn func(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error)
	deleteFn func(ctx context.Context, key string) error
	existsFn func(ctx context.Context, key string) (bool, error)
	urlFn    func(key string) string
}

func (m *mockStorage) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error) {
	return m.uploadFn(ctx, key, contentType, body, size)
}
func (m *mockStorage) Delete(ctx context.Context, key string) error { return m.deleteFn(ctx, key) }
func (m *mockStorage) Exists(ctx context.Context, key string) (bool, error) {
	return m.existsFn(ctx, key)
}
func (m *mockStorage) URL(key string) string { return m.urlFn(key) }
