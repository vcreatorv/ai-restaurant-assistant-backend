// Package indexer — единый source of truth для embed-текста и Qdrant-payload блюда.
//
// Используется как одноразовым скриптом cmd/embed-menu (массовая индексация),
// так и runtime'ом menu/usecase (точечная переиндексация после CRUD).
// Любое изменение формата embed-текста или payload-полей должно происходить здесь.
package indexer

import (
	"context"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/cohere"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

// cuisineRU маппинг кода кухни в русское слово для embed-текста.
// Не зависит от usecasemodels.Cuisine, чтобы не тащить domain-импорт в индексер.
var cuisineRU = map[string]string{
	"russian":  "русская",
	"italian":  "итальянская",
	"japanese": "японская",
	"french":   "французская",
	"asian":    "азиатская",
	"european": "европейская",
	"american": "американская",
}

// DishView — минимальный набор полей блюда, нужный индексеру.
// Заполняется и из dishRow (cmd/embed-menu), и из usecasemodels.Dish (menu/usecase).
type DishView struct {
	// ID идентификатор блюда (становится Qdrant Point ID).
	ID int
	// Name название
	Name string
	// Description описание
	Description string
	// Composition состав
	Composition string
	// Cuisine код кухни (russian, italian, ...).
	Cuisine string
	// CategoryID идентификатор категории (для payload и фильтрации).
	CategoryID int
	// CategoryName русское имя категории (входит в embed-текст).
	CategoryName string
	// Allergens коды аллергенов (для must_not-фильтра).
	Allergens []string
	// Dietary коды диетических признаков (для фильтра).
	Dietary []string
	// TagIDs идентификаторы тегов (для фильтра).
	TagIDs []int
	// PriceMinor цена в копейках (для фильтра по цене).
	PriceMinor int
	// CaloriesKcal калорийность; nil → не пишется в payload.
	CaloriesKcal *int
	// PortionWeightG вес порции; nil → не пишется в payload.
	PortionWeightG *int
	// IsAvailable доступность (для must-фильтра).
	IsAvailable bool
	// PairingTags pairing-теги для обогащения embed-текста.
	// Каждый тег — {axis, embed_phrase}; в payload Qdrant не попадают.
	PairingTags []PairingTagView
}

// PairingTagView минимальный набор полей pairing-тега, нужный индексеру:
// slug (для payload Qdrant, по нему фильтруем при hybrid retrieval),
// ось (для группировки в блоки «Подходит к / Подаётся как / Хорошо для / Тип»)
// и фраза, которая физически попадает в embed-текст.
type PairingTagView struct {
	// Slug машинный идентификатор тега (попадает в payload Qdrant как элемент массива pairing_tags)
	Slug string
	// Axis ось: drink | occasion | role | vibe (используется для группировки)
	Axis string
	// EmbedPhrase фраза в embed-тексте («белому вину»)
	EmbedPhrase string
}

// axisPrefix маппит ось pairing-тега в префикс блока в embed-тексте.
// Источник истины оси — миграция 000012_pairing_tags.up.sql (CHECK).
var axisPrefix = map[string]string{
	"drink":    "Подходит к",
	"role":     "Подаётся как",
	"occasion": "Хорошо для",
	"vibe":     "Тип",
}

// axisOrder фиксирует порядок блоков в embed-тексте, чтобы при одинаковых
// тегах две одинаковые DishView давали побайтно один embed (важно для прогнозируемого
// префикс-кэша на стороне Cohere — кэша префиксов нет, но детерминированность всё равно полезна
// для воспроизводимости тестов и дебага embed-preview).
var axisOrder = []string{"drink", "role", "occasion", "vibe"}

// BuildEmbedText собирает текст блюда для эмбеддинга.
//
// Имя категории и название кухни попадают сюда — поэтому переименование категории
// требует переиндексации всех блюд этой категории.
//
// Дальше — опциональные блоки по pairing-тегам, сгруппированные по оси
// (drink/role/occasion/vibe). Цель: дать эмбеддингу явные семантические якоря
// под пользовательские intent'ы («под белое вино», «на свидание», «накормите»).
// Cohere multilingual-v3 хорошо матчит запрос «белое вино» с фразой «белому вину»
// в embed-тексте — нормализация падежей у него встроена. Если тегов нет — блок не пишется.
func BuildEmbedText(d DishView) string {
	cuisine := cuisineRU[d.Cuisine]
	if cuisine == "" {
		cuisine = d.Cuisine
	}
	out := d.Name + "."
	if d.Description != "" {
		out += " " + d.Description
	}
	if d.Composition != "" {
		out += " Состав: " + d.Composition + "."
	}
	out += " Кухня: " + cuisine + ". Категория: " + d.CategoryName + "."

	if len(d.PairingTags) > 0 {
		out += buildPairingBlocks(d.PairingTags)
	}
	return out
}

// buildPairingBlocks группирует pairing-теги по оси и склеивает блоки вида
// «Подходит к: белому вину, игристому. Подаётся как: основное горячее.».
// Порядок осей фиксирован (axisOrder), внутри оси — порядок прихода тегов
// (предполагается, что repository отдаёт их отсортированными по sort_order).
func buildPairingBlocks(tags []PairingTagView) string {
	byAxis := make(map[string][]string, len(axisOrder))
	for _, t := range tags {
		if t.EmbedPhrase == "" {
			continue
		}
		byAxis[t.Axis] = append(byAxis[t.Axis], t.EmbedPhrase)
	}
	var sb string
	for _, axis := range axisOrder {
		phrases := byAxis[axis]
		if len(phrases) == 0 {
			continue
		}
		prefix := axisPrefix[axis]
		if prefix == "" {
			continue
		}
		sb += " " + prefix + ": " + joinComma(phrases) + "."
	}
	return sb
}

// joinComma склеивает фразы через «, » без подключения strings.Join — функция
// внутри indexer и вызывается на ~4 элемента, импорт лишнего пакета избыточен.
func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// BuildPayload собирает payload точки Qdrant — только поля для пре-фильтрации.
func BuildPayload(d DishView) map[string]any {
	p := map[string]any{
		"dish_id":      d.ID,
		"category_id":  d.CategoryID,
		"cuisine":      d.Cuisine,
		"allergens":    coalesceStrings(d.Allergens),
		"dietary":      coalesceStrings(d.Dietary),
		"tag_ids":      coalesceInts(d.TagIDs),
		"pairing_tags": pairingTagSlugs(d.PairingTags),
		"price_minor":  d.PriceMinor,
		"is_available": d.IsAvailable,
	}
	if d.CaloriesKcal != nil {
		p["calories_kcal"] = *d.CaloriesKcal
	}
	if d.PortionWeightG != nil {
		p["portion_weight_g"] = *d.PortionWeightG
	}
	return p
}

// Indexer переиндексирует блюда в Qdrant (embed + upsert).
type Indexer interface {
	// Reindex эмбеддит и upsert-ит одно блюдо.
	Reindex(ctx context.Context, d DishView) error
	// ReindexMany эмбеддит и upsert-ит несколько блюд (один батч Cohere + один батч Qdrant).
	ReindexMany(ctx context.Context, ds []DishView) error
}

// indexer стандартная реализация Indexer.
type indexer struct {
	cohere *cohere.Client
	qdrant *qdrant.Client
}

// New создаёт Indexer на базе уже инициализированных клиентов Cohere и Qdrant.
func New(co *cohere.Client, qc *qdrant.Client) Indexer {
	return &indexer{cohere: co, qdrant: qc}
}

// Reindex реиндексирует одно блюдо.
func (i *indexer) Reindex(ctx context.Context, d DishView) error {
	return i.ReindexMany(ctx, []DishView{d})
}

// ReindexMany реиндексирует несколько блюд одним батчем.
func (i *indexer) ReindexMany(ctx context.Context, ds []DishView) error {
	if len(ds) == 0 {
		return nil
	}
	texts := make([]string, len(ds))
	for k, d := range ds {
		texts[k] = BuildEmbedText(d)
	}
	vectors, err := i.cohere.Embed(ctx, texts, rag.CohereInputDocument)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if len(vectors) != len(ds) {
		return fmt.Errorf("embed: vectors %d != dishes %d", len(vectors), len(ds))
	}
	points := make([]qdrant.Point, len(ds))
	for k, d := range ds {
		points[k] = qdrant.Point{
			// dish.id из PG (SERIAL) — всегда положительный, помещается в uint64.
			ID:      uint64(d.ID), //nolint:gosec // dish_id non-negative serial
			Vector:  vectors[k],
			Payload: BuildPayload(d),
		}
	}
	if err := i.qdrant.Upsert(ctx, points); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

func coalesceStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// pairingTagSlugs извлекает slug'и из pairing-тегов для payload Qdrant.
// Используется как ключ массива «pairing_tags» в payload, по которому идёт
// hybrid retrieval: filter must=pairing_tags contains pair_white_wine.
// Пустой массив (не nil) — чтобы JSON-payload всегда имел поле, даже у блюд
// без pairing-тегов (упрощает контроль покрытия через Qdrant scroll).
func pairingTagSlugs(tags []PairingTagView) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t.Slug == "" {
			continue
		}
		out = append(out, t.Slug)
	}
	return out
}

func coalesceInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
