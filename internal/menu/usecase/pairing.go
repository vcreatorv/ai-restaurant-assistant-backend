package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

// errIndexerNotConfigured — внутренний alias для menu.ErrIndexerNotConfigured,
// чтобы внутри пакета не таскать длинное имя. Возвращается, когда админ зовёт
// PreviewDishEmbedding / DebugSearchByQuery / ReindexDish, а cohere/qdrant/indexer
// не были выставлены в конструкторе usecase. Перехватывается в HTTP-слое и маппится в 503.
var errIndexerNotConfigured = menu.ErrIndexerNotConfigured

// reindexAllBatchSize максимум блюд в одном Cohere+Qdrant батче при массовой переиндексации.
// Cohere embed-multilingual-v3 поддерживает 96, оставляем запас.
const reindexAllBatchSize = 50

// previewNeighborsDefault сколько ближайших соседей возвращать по умолчанию,
// если в PreviewDishEmbedding передан neighborsLimit <= 0.
const previewNeighborsDefault = 10

// previewVectorSample сколько компонент вектора показывать в DishEmbeddingPreview
// (sanity-check, не для серьёзного анализа).
const previewVectorSample = 8

// ListPairingTags возвращает все pairing-теги (включая неактивные) для админ UI.
func (uc *menuUsecase) ListPairingTags(ctx context.Context) ([]usecasemodels.PairingTag, error) {
	rs, err := uc.repo.ListPairingTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pairing tags: %w", err)
	}
	return usecasemodels.PairingTagsFromRepository(rs), nil
}

// SetDishPairingTags переписывает связи блюда с pairing-тегами и реиндексирует
// блюдо (embed-текст обогащается их фразами). Возвращает обновлённое блюдо.
//
// Slug'и валидируются через FK в БД (см. dish_pairing_tags.tag_slug REFERENCES
// pairing_tags.slug) — пустой/несуществующий → menu.ErrPairingTagNotFound.
func (uc *menuUsecase) SetDishPairingTags(
	ctx context.Context,
	dishID int,
	slugs []string,
) (*usecasemodels.Dish, error) {
	// Убедимся, что блюдо существует, прежде чем перезаписывать связи. Без этого
	// SetDishPairingTags(unknown_id, []) пройдёт молча — delete 0 rows + insert 0 rows.
	if _, err := uc.repo.FindDishByID(ctx, dishID); err != nil {
		return nil, err
	}
	if err := uc.repo.SetDishPairingTags(ctx, dishID, dedupSlugs(slugs)); err != nil {
		return nil, fmt.Errorf("set dish pairing tags: %w", err)
	}
	uc.reindexDish(ctx, dishID)
	return uc.GetDish(ctx, dishID)
}

// ReindexDish форсированно переиндексирует одно блюдо в Qdrant (admin-эндпоинт).
// В отличие от внутреннего reindexDish, возвращает ошибку, чтобы клиент админки
// увидел проблему с Cohere / Qdrant сразу, а не молча в логах.
func (uc *menuUsecase) ReindexDish(ctx context.Context, dishID int) error {
	if uc.indexer == nil {
		return errIndexerNotConfigured
	}
	dish, err := uc.GetDish(ctx, dishID)
	if err != nil {
		return err
	}
	view, err := uc.dishViewFor(ctx, *dish)
	if err != nil {
		return err
	}
	if err := uc.indexer.Reindex(ctx, view); err != nil {
		return fmt.Errorf("reindex dish %d: %w", dishID, err)
	}
	logger.ForCtx(ctx).Info("reindex dish (admin)", "dish_id", dishID)
	return nil
}

// ReindexAllDishes форсированно переиндексирует все блюда. При includeUnavailable=true
// в выдачу попадают и недоступные блюда (их embedding тоже хранится в Qdrant,
// is_available=false идёт в payload — для будущих фильтров и для случая, когда
// admin вернёт блюдо в продажу: оно сразу окажется в индексе с актуальным embed).
//
// Возвращает сводку (total/indexed/failed). Не падает целиком, если упал один батч.
func (uc *menuUsecase) ReindexAllDishes(
	ctx context.Context,
	includeUnavailable bool,
) (menu.DishesReindexResult, error) {
	if uc.indexer == nil {
		return menu.DishesReindexResult{}, errIndexerNotConfigured
	}
	log := logger.ForCtx(ctx)

	// Прокачиваем все блюда страницами по большому лимиту — ListDishes уже умеет
	// аллергены/диету/availability, нам нужен только availability-флаг.
	filter := usecasemodels.DishFilter{Limit: 10_000}
	if !includeUnavailable {
		t := true
		filter.Available = &t
	} else {
		// Available=nil — берём всё (репозиторий не накладывает default-фильтра
		// when Available=nil в admin контексте). Но в публичном ListDishes по
		// дефолту is_available=true. Проверим явно через UpdateDish-like обход:
		// фильтр Available=nil + не передавать → дефолт TRUE в репо. Поэтому
		// для includeUnavailable нужен явный bool-указатель на оба значения.
		// Тут проще обойти: возьмём available=true и available=false двумя проходами.
	}

	var dishes []usecasemodels.Dish
	if includeUnavailable {
		available := []bool{true, false}
		for _, a := range available {
			av := a
			f := usecasemodels.DishFilter{Limit: 10_000, Available: &av}
			batch, _, err := uc.ListDishes(ctx, f)
			if err != nil {
				return menu.DishesReindexResult{}, fmt.Errorf("list dishes (available=%v): %w", a, err)
			}
			dishes = append(dishes, batch...)
		}
	} else {
		batch, _, err := uc.ListDishes(ctx, filter)
		if err != nil {
			return menu.DishesReindexResult{}, fmt.Errorf("list dishes: %w", err)
		}
		dishes = batch
	}

	res := menu.DishesReindexResult{Total: len(dishes)}
	views := make([]indexer.DishView, 0, len(dishes))
	for _, d := range dishes {
		view, err := uc.dishViewFor(ctx, d)
		if err != nil {
			log.Warn("reindex all: build view failed", "dish_id", d.ID, "err", err)
			res.Failed++
			continue
		}
		views = append(views, view)
	}

	for start := 0; start < len(views); start += reindexAllBatchSize {
		end := start + reindexAllBatchSize
		if end > len(views) {
			end = len(views)
		}
		chunk := views[start:end]
		if err := uc.indexer.ReindexMany(ctx, chunk); err != nil {
			log.Warn("reindex all: batch failed",
				"from", start, "to", end, "err", err)
			res.Failed += len(chunk)
			continue
		}
		res.Indexed += len(chunk)
	}
	log.Info("reindex all done",
		"total", res.Total, "indexed", res.Indexed, "failed", res.Failed,
		"include_unavailable", includeUnavailable)
	return res, nil
}

// PreviewDishEmbedding возвращает embed-текст блюда, эмбедит его в Cohere и
// ищет top-N ближайших в Qdrant. neighborsLimit ≤ 0 → previewNeighborsDefault.
//
// Не требует, чтобы блюдо уже было проиндексировано: мы эмбедим текущее состояние
// (с актуальными pairing-тегами) на лету. Поэтому admin может посмотреть «как
// эмбеддинг изменится, если я добавлю этот тег» до запуска ReindexDish.
//
// Ближайшие соседи берутся без фильтров (без is_available и без allergens) —
// admin-инструмент, ему важно видеть всё пространство.
func (uc *menuUsecase) PreviewDishEmbedding(
	ctx context.Context,
	dishID, neighborsLimit int,
) (*menu.DishEmbeddingPreview, error) {
	if uc.cohere == nil || uc.qdrant == nil {
		return nil, errIndexerNotConfigured
	}
	dish, err := uc.GetDish(ctx, dishID)
	if err != nil {
		return nil, err
	}
	view, err := uc.dishViewFor(ctx, *dish)
	if err != nil {
		return nil, err
	}
	text := indexer.BuildEmbedText(view)
	vectors, err := uc.cohere.Embed(ctx, []string{text}, rag.CohereInputDocument)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("embed: empty vector")
	}
	vector := vectors[0]

	limit := neighborsLimit
	if limit <= 0 {
		limit = previewNeighborsDefault
	}
	// +1 потому что одно из top-k будет само блюдо (score≈1). Отсекать его не
	// будем — пусть админ видит «вот точка-источник», полезно для верификации.
	hits, err := uc.qdrant.Search(ctx, vector, nil, limit+1, false)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}

	neighbors, err := uc.neighborsFromHits(ctx, hits, limit+1)
	if err != nil {
		return nil, err
	}

	sample := vector
	if len(sample) > previewVectorSample {
		sample = sample[:previewVectorSample]
	}
	return &menu.DishEmbeddingPreview{
		DishID:       dishID,
		EmbedText:    text,
		VectorDim:    len(vector),
		VectorSample: sample,
		Neighbors:    neighbors,
	}, nil
}

// DebugSearchByQuery эмбеддит свободный текст и возвращает top-N ближайших
// блюд из Qdrant. Без классификатора, без rerank, без companion-логики —
// чистый вектор-поиск с фильтром is_available=true.
func (uc *menuUsecase) DebugSearchByQuery(
	ctx context.Context,
	query string,
	limit int,
) ([]menu.DishEmbeddingNeighbor, error) {
	if uc.cohere == nil || uc.qdrant == nil {
		return nil, errIndexerNotConfigured
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if limit <= 0 {
		limit = previewNeighborsDefault
	}
	vectors, err := uc.cohere.Embed(ctx, []string{query}, rag.CohereInputQuery)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embed: empty vector")
	}
	hits, err := uc.qdrant.Search(ctx, vectors[0], nil, limit, false)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}
	return uc.neighborsFromHits(ctx, hits, limit)
}

// neighborsFromHits подгружает имена/категории блюд по hits и собирает выдачу.
// Категории дочитываем через ListCategories (≤ 20 строк, в кэше pgx — копейки),
// чтобы не делать JOIN в репозитории под этот один use-case.
func (uc *menuUsecase) neighborsFromHits(
	ctx context.Context,
	hits []qdrant.SearchHit,
	limit int,
) ([]menu.DishEmbeddingNeighbor, error) {
	if len(hits) == 0 {
		return []menu.DishEmbeddingNeighbor{}, nil
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	ids := make([]int, 0, len(hits))
	scoreByID := make(map[int]float64, len(hits))
	orderByID := make(map[int]int, len(hits))
	for i, h := range hits {
		id := int(h.ID) //nolint:gosec // dish_id non-negative serial
		ids = append(ids, id)
		scoreByID[id] = h.Score
		orderByID[id] = i
	}
	dishes, err := uc.GetDishesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load neighbor dishes: %w", err)
	}
	categories, err := uc.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	catName := make(map[int]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	out := make([]menu.DishEmbeddingNeighbor, 0, len(dishes))
	for _, d := range dishes {
		out = append(out, menu.DishEmbeddingNeighbor{
			DishID:       d.ID,
			Name:         d.Name,
			CategoryName: catName[d.CategoryID],
			Score:        scoreByID[d.ID],
			IsAvailable:  d.IsAvailable,
		})
	}
	// Восстанавливаем порядок Qdrant (GetDishesByIDs его не гарантирует).
	sortByOrder(out, orderByID)
	return out, nil
}

// sortByOrder сортирует neighbors по orderByID[DishID]; на in-memory срезе из ~10
// элементов оверхед нулевой, поэтому простой O(n²) insertion sort без sort.Slice.
func sortByOrder(xs []menu.DishEmbeddingNeighbor, orderByID map[int]int) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && orderByID[xs[j].DishID] < orderByID[xs[j-1].DishID]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// dedupSlugs возвращает уникальные slug'и в порядке первого появления.
// Пустые строки отбрасываются (FK всё равно бы упал).
func dedupSlugs(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
