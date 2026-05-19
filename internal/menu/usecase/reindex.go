package usecase

import (
	"context"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// reindexBatchSize максимум блюд в одном Cohere+Qdrant батче переиндексации категории.
// Cohere embed-multilingual-v3 поддерживает до 96 текстов за запрос; 50 — безопасный потолок.
const reindexBatchSize = 50

// dishViewFor собирает indexer.DishView, дочитывая имя категории из репозитория.
//
// Имя категории нужно для embed-текста (см. indexer.BuildEmbedText). Делать JOIN на
// уровне FindDishByID было бы оптимальнее, но требует переделки domain — оставляем
// на уровне usecase (один лишний SELECT по индексу — ок).
func (uc *menuUsecase) dishViewFor(ctx context.Context, d usecasemodels.Dish) (indexer.DishView, error) {
	cat, err := uc.repo.FindCategoryByID(ctx, d.CategoryID)
	if err != nil {
		return indexer.DishView{}, fmt.Errorf("load category for dish %d: %w", d.ID, err)
	}
	tagIDs := make([]int, len(d.Tags))
	for i, t := range d.Tags {
		tagIDs[i] = t.ID
	}
	pairingTags := make([]indexer.PairingTagView, 0, len(d.PairingTags))
	for _, pt := range d.PairingTags {
		// Неактивные теги пропускаем — если админ деактивировал тег, новые
		// embed-тексты его не должны содержать. Старые векторы в Qdrant
		// перетрут на следующем реиндексе блюда.
		if !pt.IsActive {
			continue
		}
		pairingTags = append(pairingTags, indexer.PairingTagView{
			Slug:        pt.Slug,
			Axis:        pt.Axis,
			EmbedPhrase: pt.EmbedPhrase,
		})
	}
	return indexer.DishView{
		ID:             d.ID,
		Name:           d.Name,
		Description:    d.Description,
		Composition:    d.Composition,
		Cuisine:        string(d.Cuisine),
		CategoryID:     d.CategoryID,
		CategoryName:   cat.Name,
		Allergens:      d.Allergens,
		Dietary:        d.Dietary,
		TagIDs:         tagIDs,
		PriceMinor:     d.PriceMinor,
		CaloriesKcal:   d.CaloriesKcal,
		PortionWeightG: d.PortionWeightG,
		IsAvailable:    d.IsAvailable,
		PairingTags:    pairingTags,
	}, nil
}

// reindexDish переиндексирует одно блюдо в Qdrant.
//
// Если индексер не настроен (nil) — пропускает молча: usecase должен работать
// в окружениях без RAG. Ошибки индексации логируются как warn, но не возвращаются —
// CRUD-ответ клиенту не должен падать из-за временной недоступности Cohere/Qdrant.
// Восстановление консистентности при таких сбоях — через массовый make embed-menu.
func (uc *menuUsecase) reindexDish(ctx context.Context, dishID int) {
	if uc.indexer == nil {
		return
	}
	log := logger.ForCtx(ctx)
	dish, err := uc.GetDish(ctx, dishID)
	if err != nil {
		log.Warn("reindex: load dish failed", "dish_id", dishID, "err", err)
		return
	}
	view, err := uc.dishViewFor(ctx, *dish)
	if err != nil {
		log.Warn("reindex: build view failed", "dish_id", dishID, "err", err)
		return
	}
	if err := uc.indexer.Reindex(ctx, view); err != nil {
		log.Warn("reindex: indexer failed", "dish_id", dishID, "err", err)
		return
	}
	log.Info("reindex: dish indexed", "dish_id", dishID)
}

// reindexCategoryDishes переиндексирует все блюда категории.
//
// Запускается при переименовании категории — имя категории зашито в embed-текст
// (см. indexer.BuildEmbedText), поэтому без переиндексации старые векторы знают
// устаревшее имя и поиск по новому имени работает хуже.
func (uc *menuUsecase) reindexCategoryDishes(ctx context.Context, categoryID int) {
	if uc.indexer == nil {
		return
	}
	log := logger.ForCtx(ctx)
	dishes, _, err := uc.ListDishes(ctx, usecasemodels.DishFilter{CategoryID: &categoryID, Limit: 10_000})
	if err != nil {
		log.Warn("reindex category: list dishes failed", "category_id", categoryID, "err", err)
		return
	}
	if len(dishes) == 0 {
		return
	}
	views := make([]indexer.DishView, 0, len(dishes))
	for _, d := range dishes {
		view, viewErr := uc.dishViewFor(ctx, d)
		if viewErr != nil {
			log.Warn("reindex category: build view failed", "dish_id", d.ID, "err", viewErr)
			continue
		}
		views = append(views, view)
	}
	for start := 0; start < len(views); start += reindexBatchSize {
		end := start + reindexBatchSize
		if end > len(views) {
			end = len(views)
		}
		if err := uc.indexer.ReindexMany(ctx, views[start:end]); err != nil {
			log.Warn("reindex category: batch failed",
				"category_id", categoryID,
				"from", start,
				"to", end,
				"err", err,
			)
		}
	}
	log.Info("reindex category: done", "category_id", categoryID, "count", len(views))
}

// dishNeedsReindex возвращает true, если изменение блюда требует переиндексации в Qdrant.
//
// Картинка, КБЖУ-белки/жиры/углеводы и валюта в Qdrant не попадают — их изменение
// можно пропустить, экономя Cohere-вызов.
func dishNeedsReindex(oldDish, newDish *usecasemodels.Dish) bool {
	if oldDish.Name != newDish.Name ||
		oldDish.Description != newDish.Description ||
		oldDish.Composition != newDish.Composition ||
		oldDish.Cuisine != newDish.Cuisine ||
		oldDish.CategoryID != newDish.CategoryID ||
		oldDish.PriceMinor != newDish.PriceMinor ||
		oldDish.IsAvailable != newDish.IsAvailable {
		return true
	}
	if !equalIntPtr(oldDish.CaloriesKcal, newDish.CaloriesKcal) ||
		!equalIntPtr(oldDish.PortionWeightG, newDish.PortionWeightG) {
		return true
	}
	if !equalStringSlices(oldDish.Allergens, newDish.Allergens) ||
		!equalStringSlices(oldDish.Dietary, newDish.Dietary) {
		return true
	}
	if !equalTagIDs(oldDish.Tags, newDish.Tags) {
		return true
	}
	if !equalPairingSlugs(oldDish.PairingTags, newDish.PairingTags) {
		return true
	}
	return false
}

// equalPairingSlugs возвращает true, если набор slug'ов pairing-тегов идентичен.
// Порядок не важен, дубликатов быть не должно (PK в dish_pairing_tags не пустит).
func equalPairingSlugs(a, b []usecasemodels.PairingTag) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, t := range a {
		seen[t.Slug] = struct{}{}
	}
	for _, t := range b {
		if _, ok := seen[t.Slug]; !ok {
			return false
		}
	}
	return true
}

func equalIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalTagIDs(a, b []usecasemodels.Tag) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}

