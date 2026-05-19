// Package main — одноразовый скрипт индексации меню в Qdrant (Cohere embeddings).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/cmd/app/app"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/cohere"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dishRow одна строка выборки блюда из PG для индексации в Qdrant
type dishRow struct {
	// id идентификатор блюда
	id int
	// name название блюда
	name string
	// description описание блюда
	description string
	// composition состав блюда
	composition string
	// priceMinor цена в копейках
	priceMinor int
	// currency валюта (RUB, EUR)
	currency string
	// caloriesKcal калорийность, ккал
	caloriesKcal *int
	// portionWeightG вес порции, граммы
	portionWeightG *int
	// cuisine код кухни (russian, italian, ...)
	cuisine string
	// categoryID идентификатор категории
	categoryID int
	// categoryName название категории
	categoryName string
	// allergens коды аллергенов
	allergens []string
	// dietary коды диетических признаков
	dietary []string
	// isAvailable доступность блюда
	isAvailable bool
	// tagIDs идентификаторы тегов
	tagIDs []int
	// pairingTags pairing-теги для обогащения embed-текста (только активные)
	pairingTags []indexer.PairingTagView
}

// listAllDishesQuery читает все блюда с category name и tag ids одним запросом
const listAllDishesQuery = `
	SELECT
		d.id, d.name, d.description, d.composition,
		d.price_minor, d.currency,
		d.calories_kcal, d.portion_weight_g,
		d.cuisine, d.category_id, c.name AS category_name,
		d.allergens, d.dietary, d.is_available,
		COALESCE(ARRAY_AGG(dt.tag_id) FILTER (WHERE dt.tag_id IS NOT NULL), '{}') AS tag_ids
	FROM dishes d
	JOIN categories c ON c.id = d.category_id
	LEFT JOIN dish_tags dt ON dt.dish_id = d.id
	GROUP BY d.id, c.name
	ORDER BY d.id`

func main() {
	os.Exit(run())
}

func run() int {
	cfgPath := flag.String("config", app.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	cfg, err := app.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("config", "err", err)
		return 1
	}
	if rerr := validateRAG(cfg.RAG); rerr != nil {
		slog.Error("rag config", "err", rerr)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := datasources.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		slog.Error("postgres", "err", err)
		return 1
	}
	defer pool.Close()

	cohereClient, err := cohere.New(cfg.RAG.Cohere)
	if err != nil {
		slog.Error("cohere", "err", err)
		return 1
	}
	qdrantClient := qdrant.New(cfg.RAG.Qdrant)

	if cerr := qdrantClient.EnsureCollection(ctx, cfg.RAG.Cohere.EmbedDim); cerr != nil {
		slog.Error("ensure collection", "err", cerr)
		return 1
	}
	if ierr := qdrantClient.EnsurePayloadIndexes(ctx, payloadIndexes()); ierr != nil {
		slog.Error("ensure payload indexes", "err", ierr)
		return 1
	}

	dishes, err := loadDishes(ctx, pool)
	if err != nil {
		slog.Error("load dishes", "err", err)
		return 1
	}
	slog.Info("dishes loaded", "count", len(dishes))

	if ierr := indexDishes(ctx, dishes, cohereClient, qdrantClient, cfg.RAG); ierr != nil {
		slog.Error("index dishes", "err", ierr)
		return 1
	}

	total, err := qdrantClient.CountPoints(ctx)
	if err != nil {
		slog.Warn("count points", "err", err)
	}
	slog.Info("embed-menu done",
		"dishes", len(dishes),
		"qdrant_points", total,
		"collection", cfg.RAG.Qdrant.Collection,
	)
	return 0
}

// validateRAG быстрая проверка обязательных параметров RAG-конфига.
func validateRAG(c rag.Config) error {
	if c.Cohere.APIKey == "" {
		return fmt.Errorf("cohere api key is required (set COHERE_API_KEY)")
	}
	if c.Cohere.EmbedModel == "" {
		return fmt.Errorf("rag.cohere.embed_model is required")
	}
	if c.Cohere.EmbedDim <= 0 {
		return fmt.Errorf("rag.cohere.embed_dim is required")
	}
	if c.Cohere.EmbedBatchSize <= 0 {
		return fmt.Errorf("rag.cohere.embed_batch_size is required")
	}
	if c.Qdrant.URL == "" {
		return fmt.Errorf("rag.qdrant.url is required")
	}
	if c.Qdrant.Collection == "" {
		return fmt.Errorf("rag.qdrant.collection is required")
	}
	if c.Qdrant.UpsertBatchSize <= 0 {
		return fmt.Errorf("rag.qdrant.upsert_batch_size is required")
	}
	return nil
}

// payloadIndexes список payload-полей, по которым идут пре-фильтры в Qdrant
func payloadIndexes() []qdrant.PayloadIndex {
	return []qdrant.PayloadIndex{
		{Field: "category_id", Type: qdrant.FieldTypeInteger},
		{Field: "cuisine", Type: qdrant.FieldTypeKeyword},
		{Field: "allergens", Type: qdrant.FieldTypeKeyword},
		{Field: "dietary", Type: qdrant.FieldTypeKeyword},
		{Field: "tag_ids", Type: qdrant.FieldTypeInteger},
		{Field: "pairing_tags", Type: qdrant.FieldTypeKeyword},
		{Field: "price_minor", Type: qdrant.FieldTypeInteger},
		{Field: "calories_kcal", Type: qdrant.FieldTypeInteger},
		{Field: "portion_weight_g", Type: qdrant.FieldTypeInteger},
		{Field: "is_available", Type: qdrant.FieldTypeBool},
	}
}

// loadDishes читает все блюда из PG с category name и tag ids одним запросом,
// плюс отдельным проходом подгружает активные pairing-теги по dish_id.
//
// Pairing-теги отдельным запросом, а не JOIN в listAllDishesQuery, потому что
// два LEFT JOIN'а с aggregate (dish_tags + dish_pairing_tags) дают cartesian-
// взрыв и дубликаты в array_agg. Простой второй проход чище.
func loadDishes(ctx context.Context, pool *pgxpool.Pool) ([]dishRow, error) {
	rows, err := pool.Query(ctx, listAllDishesQuery)
	if err != nil {
		return nil, fmt.Errorf("query dishes: %w", err)
	}
	defer rows.Close()

	var out []dishRow
	for rows.Next() {
		var d dishRow
		if err := rows.Scan(
			&d.id, &d.name, &d.description, &d.composition,
			&d.priceMinor, &d.currency,
			&d.caloriesKcal, &d.portionWeightG,
			&d.cuisine, &d.categoryID, &d.categoryName,
			&d.allergens, &d.dietary, &d.isAvailable,
			&d.tagIDs,
		); err != nil {
			return nil, fmt.Errorf("scan dish: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows dishes: %w", err)
	}

	pairings, err := loadPairingTagsByDish(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].pairingTags = pairings[out[i].id]
	}
	return out, nil
}

// loadPairingTagsByDish возвращает map dish_id → []PairingTagView (только активные
// теги). Один запрос на всю таблицу — для меню в ~200 блюд / 20 тегов это копейки,
// группировка в Go.
func loadPairingTagsByDish(ctx context.Context, pool *pgxpool.Pool) (map[int][]indexer.PairingTagView, error) {
	const q = `
		SELECT dpt.dish_id, pt.slug, pt.axis, pt.embed_phrase
		FROM dish_pairing_tags dpt
		JOIN pairing_tags pt ON pt.slug = dpt.tag_slug
		WHERE pt.is_active
		ORDER BY dpt.dish_id, pt.axis, pt.sort_order, pt.slug`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query pairing_tags by dish: %w", err)
	}
	defer rows.Close()

	out := map[int][]indexer.PairingTagView{}
	for rows.Next() {
		var dishID int
		var v indexer.PairingTagView
		if err := rows.Scan(&dishID, &v.Slug, &v.Axis, &v.EmbedPhrase); err != nil {
			return nil, fmt.Errorf("scan pairing: %w", err)
		}
		out[dishID] = append(out[dishID], v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows pairing: %w", err)
	}
	return out, nil
}

// indexDishes эмбеддит и upsert-ит блюда батчами через общий indexer.
//
// Размер батча — минимум из cohere.embed_batch_size и qdrant.upsert_batch_size.
// Логика embed-текста и payload живёт в internal/menu/indexer, чтобы не разъезжаться
// с runtime'ом из menu/usecase.
func indexDishes(
	ctx context.Context,
	dishes []dishRow,
	co *cohere.Client,
	qc *qdrant.Client,
	cfg rag.Config,
) error {
	batch := cfg.Cohere.EmbedBatchSize
	if cfg.Qdrant.UpsertBatchSize < batch {
		batch = cfg.Qdrant.UpsertBatchSize
	}

	idx := indexer.New(co, qc)

	for start := 0; start < len(dishes); start += batch {
		end := start + batch
		if end > len(dishes) {
			end = len(dishes)
		}
		chunk := dishes[start:end]
		views := make([]indexer.DishView, len(chunk))
		for i, d := range chunk {
			views[i] = dishRowToView(d)
		}
		if err := idx.ReindexMany(ctx, views); err != nil {
			return fmt.Errorf("reindex batch [%d:%d]: %w", start, end, err)
		}
		slog.Info("indexed batch", "from", start, "to", end, "size", len(chunk))
	}
	return nil
}

// dishRowToView маппит локальную строку выборки PG в нейтральный indexer.DishView.
func dishRowToView(d dishRow) indexer.DishView {
	return indexer.DishView{
		ID:             d.id,
		Name:           d.name,
		Description:    d.description,
		Composition:    d.composition,
		Cuisine:        d.cuisine,
		CategoryID:     d.categoryID,
		CategoryName:   d.categoryName,
		Allergens:      d.allergens,
		Dietary:        d.dietary,
		TagIDs:         d.tagIDs,
		PriceMinor:     d.priceMinor,
		CaloriesKcal:   d.caloriesKcal,
		PortionWeightG: d.portionWeightG,
		IsAvailable:    d.isAvailable,
		PairingTags:    d.pairingTags,
	}
}
