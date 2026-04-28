package menu

import (
	"context"
	"errors"
	"io"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

var (
	// ErrCategoryNotFound категория не найдена
	ErrCategoryNotFound = errors.New("category not found")
	// ErrCategoryNameTaken категория с таким именем уже существует
	ErrCategoryNameTaken = errors.New("category name already taken")
	// ErrCategoryHasDishes в категории есть блюда — удалить нельзя
	ErrCategoryHasDishes = errors.New("category has dishes")

	// ErrTagNotFound тег не найден
	ErrTagNotFound = errors.New("tag not found")
	// ErrTagNameTaken тег с таким именем или slug уже существует
	ErrTagNameTaken = errors.New("tag name or slug already taken")

	// ErrDishNotFound блюдо не найдено
	ErrDishNotFound = errors.New("dish not found")
	// ErrDishNameTaken блюдо с таким именем уже существует
	ErrDishNameTaken = errors.New("dish name already taken")
	// ErrInvalidCuisine недопустимое значение cuisine
	ErrInvalidCuisine = errors.New("invalid cuisine")
)

// Usecase сценарии работы с меню
type Usecase interface {
	// ListCategories возвращает все доступные категории, отсортированные по sort_order
	ListCategories(ctx context.Context) ([]usecasemodels.Category, error)
	// CreateCategory создаёт категорию
	CreateCategory(ctx context.Context, c usecasemodels.CategoryCreate) (*usecasemodels.Category, error)
	// UpdateCategory обновляет категорию
	UpdateCategory(ctx context.Context, id int, p usecasemodels.CategoryPatch) (*usecasemodels.Category, error)
	// DeleteCategory удаляет категорию
	DeleteCategory(ctx context.Context, id int) error

	// ListTags возвращает все теги
	ListTags(ctx context.Context) ([]usecasemodels.Tag, error)
	// CreateTag создаёт тег
	CreateTag(ctx context.Context, t usecasemodels.TagCreate) (*usecasemodels.Tag, error)
	// UpdateTag обновляет тег
	UpdateTag(ctx context.Context, id int, p usecasemodels.TagPatch) (*usecasemodels.Tag, error)
	// DeleteTag удаляет тег
	DeleteTag(ctx context.Context, id int) error

	// ListDishes возвращает блюда с фильтрами и пагинацией
	ListDishes(ctx context.Context, f usecasemodels.DishFilter) ([]usecasemodels.Dish, int, error)
	// GetDish возвращает блюдо по идентификатору
	GetDish(ctx context.Context, id int) (*usecasemodels.Dish, error)
	// CreateDish создаёт блюдо
	CreateDish(ctx context.Context, d usecasemodels.DishCreate) (*usecasemodels.Dish, error)
	// UpdateDish обновляет блюдо
	UpdateDish(ctx context.Context, id int, p usecasemodels.DishPatch) (*usecasemodels.Dish, error)
	// DeleteDish помечает блюдо недоступным (soft delete)
	DeleteDish(ctx context.Context, id int) error
	// UploadDishImage заливает картинку блюда в S3 и сохраняет URL
	UploadDishImage(ctx context.Context, id int, src DishImageSource) (*usecasemodels.Dish, error)
}

// DishImageSource исходные данные картинки блюда
type DishImageSource struct {
	// Body тело файла
	Body io.Reader
	// ContentType mime-type (image/jpeg, image/png, image/webp)
	ContentType string
	// Size размер в байтах
	Size int64
	// Ext расширение без точки (jpg, png, webp)
	Ext string
}

var (
	// ErrImageTooLarge файл превышает допустимый размер
	ErrImageTooLarge = errors.New("image too large")
	// ErrImageUnsupportedType неподдерживаемый mime-type
	ErrImageUnsupportedType = errors.New("image unsupported type")
)

// Repository хранилище меню
type Repository interface {
	// ListCategories возвращает все категории
	ListCategories(ctx context.Context) ([]repositorymodels.Category, error)
	// FindCategoryByID возвращает категорию по идентификатору
	FindCategoryByID(ctx context.Context, id int) (*repositorymodels.Category, error)
	// CreateCategory вставляет новую категорию
	CreateCategory(ctx context.Context, c *repositorymodels.Category) error
	// UpdateCategory сохраняет изменения категории
	UpdateCategory(ctx context.Context, c *repositorymodels.Category) error
	// DeleteCategory удаляет категорию
	DeleteCategory(ctx context.Context, id int) error

	// ListTags возвращает все теги
	ListTags(ctx context.Context) ([]repositorymodels.Tag, error)
	// FindTagsByIDs возвращает теги по списку идентификаторов
	FindTagsByIDs(ctx context.Context, ids []int) ([]repositorymodels.Tag, error)
	// FindTagByID возвращает тег по идентификатору
	FindTagByID(ctx context.Context, id int) (*repositorymodels.Tag, error)
	// CreateTag вставляет новый тег
	CreateTag(ctx context.Context, t *repositorymodels.Tag) error
	// UpdateTag сохраняет изменения тега
	UpdateTag(ctx context.Context, t *repositorymodels.Tag) error
	// DeleteTag удаляет тег
	DeleteTag(ctx context.Context, id int) error

	// ListDishes возвращает блюда по фильтру + общий count
	ListDishes(ctx context.Context, f repositorymodels.DishFilter) ([]repositorymodels.Dish, int, error)
	// FindDishByID возвращает блюдо вместе с тегами
	FindDishByID(ctx context.Context, id int) (*repositorymodels.Dish, error)
	// CreateDish вставляет блюдо с тегами в транзакции
	CreateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	// UpdateDish обновляет блюдо и (если tagIDs != nil) перепривязывает теги
	UpdateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	// SetDishAvailability обновляет is_available
	SetDishAvailability(ctx context.Context, id int, available bool) error
}
