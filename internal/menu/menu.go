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
	// ErrInvalidCategoryRole недопустимое значение role
	ErrInvalidCategoryRole = errors.New("invalid category role")

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

	// ErrPairingTagNotFound один или несколько pairing-тегов не существуют в vocabulary
	ErrPairingTagNotFound = errors.New("pairing tag not found")
	// ErrIndexerNotConfigured admin-операция (reindex / embed-preview / debug-search) запрошена
	// в окружении, где Cohere/Qdrant/Indexer не выставлены (типично — dev-режим без RAG).
	// HTTP-слой маппит в 503.
	ErrIndexerNotConfigured = errors.New("indexer not configured")
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
	// GetDishesByIDs batch-возврат блюд (порядок результатов не гарантирован)
	GetDishesByIDs(ctx context.Context, ids []int) ([]usecasemodels.Dish, error)
	// CreateDish создаёт блюдо
	CreateDish(ctx context.Context, d usecasemodels.DishCreate) (*usecasemodels.Dish, error)
	// UpdateDish обновляет блюдо
	UpdateDish(ctx context.Context, id int, p usecasemodels.DishPatch) (*usecasemodels.Dish, error)
	// DeleteDish помечает блюдо недоступным (soft delete)
	DeleteDish(ctx context.Context, id int) error
	// UploadDishImage заливает картинку блюда в S3 и сохраняет URL
	UploadDishImage(ctx context.Context, id int, src DishImageSource) (*usecasemodels.Dish, error)

	// ListPairingTags возвращает все pairing-теги (включая неактивные) — для админ UI
	ListPairingTags(ctx context.Context) ([]usecasemodels.PairingTag, error)
	// SetDishPairingTags переписывает связи блюда с pairing-тегами (delete-all + insert)
	// и запускает реиндекс блюда.
	SetDishPairingTags(ctx context.Context, dishID int, slugs []string) (*usecasemodels.Dish, error)

	// ReindexDish форсированно переиндексирует одно блюдо в Qdrant. Возвращает ошибку,
	// если индексер не настроен (admin контекст должен это знать).
	ReindexDish(ctx context.Context, dishID int) error
	// ReindexAllDishes переиндексирует все блюда. Включать недоступные опционально —
	// они нужны для будущих фильтров (например, если админ снимет недоступность,
	// блюдо сразу окажется в индексе с актуальным эмбеддингом).
	ReindexAllDishes(ctx context.Context, includeUnavailable bool) (DishesReindexResult, error)

	// PreviewDishEmbedding возвращает: embed-текст блюда, размерность вектора,
	// сэмпл первых значений, top-N ближайших соседей в Qdrant. neighborsLimit ≤ 0
	// → дефолт 10. Не требует, чтобы блюдо было предварительно проиндексировано.
	PreviewDishEmbedding(ctx context.Context, dishID, neighborsLimit int) (*DishEmbeddingPreview, error)
	// DebugSearchByQuery эмбеддит произвольный текст и возвращает top-N блюд из Qdrant
	// (без классификатора, без rerank, без companion-логики — голый вектор-поиск).
	// Полезно для отладки RAG-промахов: «как retrieval отрабатывает на этот запрос».
	DebugSearchByQuery(ctx context.Context, query string, limit int) ([]DishEmbeddingNeighbor, error)
}

// DishesReindexResult сводка по массовой переиндексации.
type DishesReindexResult struct {
	// Total сколько всего блюд было обработано (попало в батчи)
	Total int
	// Indexed сколько успешно прошло Cohere+Qdrant
	Indexed int
	// Failed сколько провалилось (логи в warn, ошибки не возвращаются индивидуально)
	Failed int
}

// DishEmbeddingPreview ответ PreviewDishEmbedding для админ UI.
type DishEmbeddingPreview struct {
	// DishID идентификатор исходного блюда
	DishID int
	// EmbedText финальный текст, который ушёл в Cohere (после BuildEmbedText)
	EmbedText string
	// VectorDim размерность embedding-вектора
	VectorDim int
	// VectorSample первые 8 значений вектора — для sanity-check, что Cohere вернул осмысленный
	VectorSample []float32
	// Neighbors top-N ближайших точек в Qdrant (включая само блюдо со score≈1)
	Neighbors []DishEmbeddingNeighbor
}

// DishEmbeddingNeighbor одна точка в выдаче PreviewDishEmbedding / DebugSearchByQuery.
type DishEmbeddingNeighbor struct {
	// DishID id блюда в PG
	DishID int
	// Name название блюда
	Name string
	// CategoryName категория (русское имя)
	CategoryName string
	// Score cosine-similarity (выше = ближе)
	Score float64
	// IsAvailable доступно ли блюдо (полезно при дебаге — показать «выключенные»)
	IsAvailable bool
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
	// FindDishesByIDs batch-возврат блюд по списку идентификаторов
	FindDishesByIDs(ctx context.Context, ids []int) ([]repositorymodels.Dish, error)
	// CreateDish вставляет блюдо с тегами в транзакции
	CreateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	// UpdateDish обновляет блюдо и (если tagIDs != nil) перепривязывает теги
	UpdateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error
	// SetDishAvailability обновляет is_available
	SetDishAvailability(ctx context.Context, id int, available bool) error

	// ListPairingTags возвращает все pairing-теги (включая неактивные) — для админ UI
	ListPairingTags(ctx context.Context) ([]repositorymodels.PairingTag, error)
	// ListActivePairingTags возвращает только активные (для валидации и формы редактирования)
	ListActivePairingTags(ctx context.Context) ([]repositorymodels.PairingTag, error)
	// FindPairingTagsBySlugs возвращает теги по списку slug'ов (порядок не гарантирован)
	FindPairingTagsBySlugs(ctx context.Context, slugs []string) ([]repositorymodels.PairingTag, error)
	// PairingTagsForDish возвращает pairing-теги одного блюда
	PairingTagsForDish(ctx context.Context, dishID int) ([]repositorymodels.PairingTag, error)
	// SetDishPairingTags переписывает связи блюда с pairing-тегами (delete-all + insert).
	// При несуществующем slug возвращает ErrPairingTagNotFound.
	SetDishPairingTags(ctx context.Context, dishID int, slugs []string) error
	// DishIDsByPairingTag id блюд с конкретным тегом (для каскадного реиндекса)
	DishIDsByPairingTag(ctx context.Context, slug string) ([]int, error)
}
