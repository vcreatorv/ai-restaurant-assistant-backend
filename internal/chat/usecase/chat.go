package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"

	"github.com/google/uuid"
)

// cuisineRU маппинг enum-кода кухни в русское слово для prompt'а
var cuisineRU = map[string]string{
	"russian":  "русская",
	"italian":  "итальянская",
	"japanese": "японская",
	"french":   "французская",
	"asian":    "азиатская",
	"european": "европейская",
	"american": "американская",
}

// jsonFencedRe ловит JSON-блок, обёрнутый в ```json ... ``` (рекомендуемый формат от LLM)
var jsonFencedRe = regexp.MustCompile("(?s)```(?:json)?\\s*({[^`]*?})\\s*```")

// jsonTailRe ловит голый JSON-объект с recommended_dish_ids в самом конце ответа LLM
var jsonTailRe = regexp.MustCompile(`(?s)\{[^{}]*"recommended_dish_ids"[^{}]*\}\s*$`)

// GetActive возвращает активный чат пользователя; создаёт новый, если последний устарел или отсутствует
func (uc *chatUsecase) GetActive(ctx context.Context, userID uuid.UUID) (*usecasemodels.Chat, error) {
	raw, err := uc.repo.FindMostRecentByUser(ctx, userID)
	if err == nil {
		if !uc.isStale(raw.LastMessageAt) {
			return usecasemodels.ChatFromRepository(raw), nil
		}
	} else if !errors.Is(err, chat.ErrChatNotFound) {
		return nil, fmt.Errorf("find recent chat: %w", err)
	}
	return uc.Create(ctx, userID, nil)
}

// List возвращает чаты пользователя
func (uc *chatUsecase) List(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]usecasemodels.Chat, int, error) {
	items, total, err := uc.repo.ListChatsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list chats: %w", err)
	}
	return usecasemodels.ChatsFromRepository(items), total, nil
}

// Create создаёт чат с опциональным заголовком
func (uc *chatUsecase) Create(
	ctx context.Context,
	userID uuid.UUID,
	title *string,
) (*usecasemodels.Chat, error) {
	c := &repositorymodels.Chat{
		ID:     uc.uuid.New(),
		UserID: userID,
		Title:  title,
	}
	if err := uc.repo.CreateChat(ctx, c); err != nil {
		return nil, fmt.Errorf("create chat: %w", err)
	}
	return usecasemodels.ChatFromRepository(c), nil
}

// GetWithMessages возвращает чат и сообщения
func (uc *chatUsecase) GetWithMessages(
	ctx context.Context,
	userID, chatID uuid.UUID,
	limit int,
	before *uuid.UUID,
) (*usecasemodels.Chat, []usecasemodels.Message, bool, error) {
	c, err := uc.loadChatOwned(ctx, userID, chatID)
	if err != nil {
		return nil, nil, false, err
	}
	rawMessages, hasMore, err := uc.repo.ListMessages(ctx, chatID, limit, before)
	if err != nil {
		return nil, nil, false, fmt.Errorf("list messages: %w", err)
	}
	reverseMessages(rawMessages)
	return usecasemodels.ChatFromRepository(c), usecasemodels.MessagesFromRepository(rawMessages), hasMore, nil
}

// Delete удаляет чат пользователя
func (uc *chatUsecase) Delete(ctx context.Context, userID, chatID uuid.UUID) error {
	if _, err := uc.loadChatOwned(ctx, userID, chatID); err != nil {
		return err
	}
	return uc.repo.DeleteChat(ctx, chatID)
}

// SendMessage прогоняет RAG-pipeline и стримит ответ ассистента через cb
func (uc *chatUsecase) SendMessage(
	ctx context.Context,
	userID, chatID uuid.UUID,
	content string,
	cb chat.SendCallbacks,
) error {
	if strings.TrimSpace(content) == "" {
		return chat.ErrEmptyMessage
	}
	if _, err := uc.loadChatOwned(ctx, userID, chatID); err != nil {
		return err
	}

	log := logger.ForCtx(ctx).With(
		"chat_id", chatID,
		"user_id", userID,
	)
	log.Info("chat pipeline start",
		"stage", "begin",
		"content_len", len(content),
		"content_preview", previewText(content, 80),
	)
	start := time.Now()

	// 1. user-message сразу в БД (чтобы не потерять при сбое LLM).
	userMsg := &repositorymodels.Message{
		ID:                 uc.uuid.New(),
		ChatID:             chatID,
		Role:               string(usecasemodels.RoleUser),
		Content:            content,
		RecommendedDishIDs: []int{},
		Meta:               map[string]any{},
	}
	if err := uc.repo.AppendMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("append user message: %w", err)
	}

	// 2. Профиль пользователя (для pre-filter и prompt'а).
	profile, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load profile: %w", err)
	}

	// 3. История диалога: anchor (первый user-msg чата) + последние N пар.
	hist, err := uc.loadHistory(ctx, chatID, userMsg.ID)
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	log.Debug("chat history loaded",
		"stage", "history",
		"anchor_present", hist.anchor != nil,
		"recent_count", len(hist.recent),
		"prior_recommended", hist.priorRecommended,
	)

	// 4. RAG: embed → search → rerank → load full data + companions.
	rag, ragErr := uc.runRAG(ctx, content, profile)
	if ragErr != nil {
		log.Error("chat rag stage failed", "stage", "rag", "err", ragErr)
		return ragErr
	}
	log.Info("chat rag retrieved",
		"stage", "rag",
		"retrieved_count", len(rag.retrievedIDs),
		"retrieved_ids", rag.retrievedIDs,
		"reranked_ids", rag.rerankedIDs,
		"companion_ids", companionIDs(rag.companions),
	)

	// 5. Готовим metadata для клиента и assistant-сообщения.
	assistantMessageID := uc.uuid.New()
	if cb.OnMeta != nil {
		if err := cb.OnMeta(chat.MetaEvent{
			MessageID:          assistantMessageID,
			RecommendedDishIDs: rag.rerankedIDs,
		}); err != nil {
			return fmt.Errorf("on meta: %w", err)
		}
	}

	// 6. Promp + LLM stream. Накопленный текст пишем в буфер, токены отдаём клиенту.
	prompt := buildPrompt(promptInput{
		profile:          profile,
		retrieved:        rag.main,
		companions:       rag.companions,
		priorRecommended: hist.priorRecommended,
		anchor:           hist.anchor,
		history:          hist.recent,
		currentUserText:  content,
	})
	llmStart := time.Now()
	log.Debug("chat llm prompt assembled",
		"stage", "llm.prompt",
		"messages_count", len(prompt),
		"main_dishes", len(rag.main),
		"companions", len(rag.companions),
	)
	var fullText strings.Builder
	usage, llmErr := uc.llm.ChatStream(ctx, llm.ChatRequest{Messages: prompt}, func(delta string) error {
		fullText.WriteString(delta)
		if cb.OnDelta != nil {
			return cb.OnDelta(delta)
		}
		return nil
	})
	if llmErr != nil {
		log.Error("chat llm stream failed",
			"stage", "llm.stream",
			"err", llmErr,
			"llm_ms", time.Since(llmStart).Milliseconds(),
		)
		return fmt.Errorf("%w: %s", chat.ErrUpstreamFailure, llmErr.Error())
	}
	log.Info("chat llm response",
		"stage", "llm.stream",
		"llm_ms", time.Since(llmStart).Milliseconds(),
		"tokens_in", usage.PromptTokens,
		"tokens_out", usage.CompletionTokens,
		"finish_reason", usage.FinishReason,
		"model_actual", usage.Model,
		"text_len", fullText.Len(),
	)
	if usage.FinishReason == "length" {
		log.Warn("chat llm truncated (finish_reason=length)",
			"stage", "llm.stream",
			"hint", "ответ обрезан max_tokens; JSON-блок recommended_dish_ids "+
				"скорее всего не дошёл — увеличь rag.llm.common.max_tokens или попробуй другую модель",
			"tokens_out", usage.CompletionTokens,
			"text_len", fullText.Len(),
		)
	}

	// 7. Парсим JSON-блок ассистента, чистим текст для сохранения и отдачи.
	cleanText, llmRecommended := parseLLMTail(fullText.String())
	log.Debug("chat llm parsed",
		"stage", "llm.parse",
		"clean_text_preview", previewText(cleanText, 120),
		"llm_recommended", llmRecommended,
	)

	// recommended_dish_ids в API/БД отражает то, что LLM реально упомянула в ответе.
	// От рерэнкера в meta остаётся отдельное поле reranked_ids — для аналитики.
	finalRecommended := llmRecommended
	if finalRecommended == nil {
		finalRecommended = []int{}
	}

	// 8. Assistant-message в БД с полной телеметрией.
	assistantMeta := map[string]any{
		"latency_ms":    time.Since(start).Milliseconds(),
		"tokens_in":     usage.PromptTokens,
		"tokens_out":    usage.CompletionTokens,
		"finish_reason": usage.FinishReason,
		"model":         usage.Model,
		"retrieved_ids": rag.retrievedIDs,
		"reranked_ids":  rag.rerankedIDs,
		"companion_ids": companionIDs(rag.companions),
	}
	assistantMsg := &repositorymodels.Message{
		ID:                 assistantMessageID,
		ChatID:             chatID,
		Role:               string(usecasemodels.RoleAssistant),
		Content:            cleanText,
		RecommendedDishIDs: finalRecommended,
		Meta:               assistantMeta,
	}
	if err := uc.repo.AppendMessage(ctx, assistantMsg); err != nil {
		return fmt.Errorf("append assistant message: %w", err)
	}

	// 9. Финальный done.
	if cb.OnDone != nil {
		if err := cb.OnDone(chat.DoneEvent{
			LatencyMS: time.Since(start).Milliseconds(),
			TokensIn:  usage.PromptTokens,
			TokensOut: usage.CompletionTokens,
			Model:     usage.Model,
		}); err != nil {
			return fmt.Errorf("on done: %w", err)
		}
	}
	return nil
}

// ragResult содержит главные блюда + сопровождение + телеметрию retrieval-стадии
type ragResult struct {
	// main блюда из основного retrieval (top-N после rerank)
	main []retrievedDish
	// companions по 1 блюду из каждой companion-категории (соус/гарнир/десерт/напиток/...)
	companions []retrievedDish
	// retrievedIDs все id, поднятые primary-search'ем (для аналитики)
	retrievedIDs []int
	// rerankedIDs id после rerank, в порядке убывания релевантности
	rerankedIDs []int
}

// runRAG выполняет embed → primary search/rerank → companion searches → load.
// Embed и rerank работают только с текущим сообщением — иначе разнотемные предыдущие
// вопросы смазывают вектор и rerank теряет фокус. Контекст диалога LLM получает
// отдельно через массив messages.
func (uc *chatUsecase) runRAG(
	ctx context.Context,
	currentMessage string,
	profile *usecasemodels.User,
) (ragResult, error) {
	log := logger.ForCtx(ctx)

	t0 := time.Now()
	embeds, err := uc.cohere.Embed(ctx, []string{currentMessage}, rag.CohereInputQuery)
	if err != nil {
		return ragResult{}, fmt.Errorf("%w: embed: %s", chat.ErrUpstreamFailure, err.Error())
	}
	if len(embeds) == 0 {
		return ragResult{}, fmt.Errorf("%w: empty embed", chat.ErrUpstreamFailure)
	}
	embed := embeds[0]
	log.Debug("rag embed done",
		"stage", "rag.embed",
		"embed_ms", time.Since(t0).Milliseconds(),
		"dim", len(embed),
	)

	prefilter := buildPrefilter(profile)
	hits, err := uc.qdrant.Search(ctx, embed, prefilter, uc.ragCfg.Search.TopK, true)
	if err != nil {
		return ragResult{}, fmt.Errorf("%w: search: %s", chat.ErrUpstreamFailure, err.Error())
	}
	log.Debug("rag primary search done",
		"stage", "rag.search.primary",
		"hits", len(hits),
		"top_score", topScore(hits),
	)

	categories, err := uc.menu.ListCategories(ctx)
	if err != nil {
		return ragResult{}, fmt.Errorf("list categories: %w", err)
	}
	catName := make(map[int]string, len(categories))
	catIDByName := make(map[string]int, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
		catIDByName[c.Name] = c.ID
	}

	if len(hits) == 0 {
		log.Warn("rag search returned no hits", "prefilter", prefilter)
		return ragResult{retrievedIDs: []int{}, rerankedIDs: []int{}}, nil
	}

	retrievedIDs := make([]int, len(hits))
	for i, h := range hits {
		// h.ID из Qdrant — это dish_id (SERIAL из PG), помещается в int
		retrievedIDs[i] = int(h.ID) //nolint:gosec // dish_id fits in int
	}

	dishes, err := uc.menu.GetDishesByIDs(ctx, retrievedIDs)
	if err != nil {
		return ragResult{}, fmt.Errorf("load dishes: %w", err)
	}
	dishesByID := make(map[int]usecasemodels.Dish, len(dishes))
	for _, d := range dishes {
		dishesByID[d.ID] = d
	}

	rerankInput := make([]string, 0, len(hits))
	rerankIDs := make([]int, 0, len(hits))
	for _, h := range hits {
		d, ok := dishesByID[int(h.ID)] //nolint:gosec // dish_id fits in int
		if !ok {
			continue
		}
		rerankInput = append(rerankInput, dishToText(&d))
		rerankIDs = append(rerankIDs, d.ID)
	}

	t2 := time.Now()
	rerankedIDs := uc.rerankOrFallback(ctx, currentMessage, rerankInput, rerankIDs)
	log.Debug("rag rerank done",
		"stage", "rag.rerank",
		"rerank_ms", time.Since(t2).Milliseconds(),
		"input", len(rerankInput),
		"output", len(rerankedIDs),
	)

	main := buildRetrievedDishes(rerankedIDs, dishesByID, catName)
	mainSet := make(map[int]struct{}, len(rerankedIDs))
	for _, id := range rerankedIDs {
		mainSet[id] = struct{}{}
	}
	mainCatSet := make(map[int]struct{}, len(main))
	for _, d := range main {
		if catID, ok := catIDByName[d.categoryName]; ok {
			mainCatSet[catID] = struct{}{}
		}
	}

	// Диверсификация main по категориям (если в reranked top-N покрыто мало
	// разных main-категорий — добавляем top-1 из непокрытых).
	diversified := uc.runMainDiversification(ctx, embed, prefilter, mainSet, mainCatSet, catIDByName, catName)
	for _, d := range diversified {
		main = append(main, d)
		mainSet[d.id] = struct{}{}
		if catID, ok := catIDByName[d.categoryName]; ok {
			mainCatSet[catID] = struct{}{}
		}
	}

	companions := uc.runCompanions(ctx, embed, prefilter, mainSet, mainCatSet, catIDByName, catName)

	return ragResult{
		main:         main,
		companions:   companions,
		retrievedIDs: retrievedIDs,
		rerankedIDs:  rerankedIDs,
	}, nil
}

// runMainDiversification на широких запросах добавляет в main top-1 из тех
// main-категорий (rag.chat.main_categories), которые не покрыты reranked top-N.
// Срабатывает только если: (а) в main < cfg.MainMinCategories разных main-категорий,
// (б) score top-1 >= cfg.MainDiversifyMinScore (на узком запросе нерелевантное не подсосётся).
// Лимит cfg.MainMaxAdded.
func (uc *chatUsecase) runMainDiversification(
	ctx context.Context,
	embed []float32,
	base *qdrant.Filter,
	mainSet map[int]struct{},
	mainCatSet map[int]struct{},
	catIDByName map[string]int,
	catName map[int]string,
) []retrievedDish {
	log := logger.ForCtx(ctx)
	names := uc.ragCfg.Chat.MainCategories
	minCats := uc.ragCfg.Chat.MainMinCategories
	maxAdded := uc.ragCfg.Chat.MainMaxAdded
	minScore := uc.ragCfg.Chat.MainDiversifyMinScore
	if len(names) == 0 || maxAdded <= 0 {
		return nil
	}

	// Считаем покрытые main-категории (только те, что есть в нашем list main_categories)
	covered := 0
	mainCatNames := make(map[string]struct{}, len(names))
	for _, n := range names {
		mainCatNames[n] = struct{}{}
	}
	for catID := range mainCatSet {
		if _, ok := mainCatNames[catName[catID]]; ok {
			covered++
		}
	}
	log.Debug("rag main coverage",
		"stage", "rag.diversify",
		"covered_main_categories", covered,
		"min_required", minCats,
	)
	if covered >= minCats {
		return nil
	}

	collected := make([]int, 0, maxAdded)
	for _, name := range names {
		if len(collected) >= maxAdded {
			break
		}
		catID, ok := catIDByName[name]
		if !ok {
			log.Warn("rag diversify category not found", "name", name)
			continue
		}
		if _, alreadyCovered := mainCatSet[catID]; alreadyCovered {
			continue
		}
		filter := withCategoryID(base, catID)

		t := time.Now()
		hits, err := uc.qdrant.Search(ctx, embed, filter, 1, false)
		if err != nil {
			log.Warn("rag diversify search failed",
				"stage", "rag.diversify",
				"category", name,
				"err", err,
			)
			continue
		}
		if len(hits) == 0 {
			continue
		}
		log.Debug("rag diversify search done",
			"stage", "rag.diversify",
			"category", name,
			"category_id", catID,
			"top_score", hits[0].Score,
			"min_score", minScore,
			"search_ms", time.Since(t).Milliseconds(),
		)
		if hits[0].Score < minScore {
			continue
		}
		id := int(hits[0].ID) //nolint:gosec // dish_id fits in int
		if _, dup := mainSet[id]; dup {
			continue
		}
		collected = append(collected, id)
	}

	if len(collected) == 0 {
		return nil
	}
	dishes, err := uc.menu.GetDishesByIDs(ctx, collected)
	if err != nil {
		log.Warn("rag diversify load dishes failed", "err", err)
		return nil
	}
	dishesByID := make(map[int]usecasemodels.Dish, len(dishes))
	for _, d := range dishes {
		dishesByID[d.ID] = d
	}
	out := buildRetrievedDishes(collected, dishesByID, catName)
	log.Info("rag main diversified",
		"stage", "rag.diversify",
		"added_ids", collected,
		"added_count", len(out),
	)
	return out
}

// runCompanions для каждой companion-категории из конфига делает Qdrant.Search
// с pre-filter (профиль + is_available + category_id) и берёт top-1, дедупит с main.
// Без rerank: companion — это «лучшая семантически близкая позиция в категории»,
// ради точности нет смысла платить ещё один Cohere-вызов.
//
// Дедуп: пропускаем категорию, если main уже содержит блюдо оттуда (mainCatSet),
// чтобы не получалось «два хлеба» / «два соуса» в ответе LLM.
func (uc *chatUsecase) runCompanions(
	ctx context.Context,
	embed []float32,
	base *qdrant.Filter,
	mainSet map[int]struct{},
	mainCatSet map[int]struct{},
	catIDByName map[string]int,
	catName map[int]string,
) []retrievedDish {
	log := logger.ForCtx(ctx)
	names := uc.ragCfg.Chat.Companions
	if len(names) == 0 {
		return nil
	}

	used := make(map[int]struct{}, len(mainSet)+len(names))
	for id := range mainSet {
		used[id] = struct{}{}
	}

	collected := make([]int, 0, len(names))
	for _, name := range names {
		catID, ok := catIDByName[name]
		if !ok {
			log.Warn("companion category not found", "name", name)
			continue
		}
		if _, exists := mainCatSet[catID]; exists {
			log.Debug("rag companion skipped (category already in main)",
				"stage", "rag.search.companion",
				"category", name,
				"category_id", catID,
			)
			continue
		}
		filter := withCategoryID(base, catID)

		t := time.Now()
		hits, err := uc.qdrant.Search(ctx, embed, filter, 5, false)
		if err != nil {
			log.Warn("rag companion search failed",
				"stage", "rag.search.companion",
				"category", name,
				"category_id", catID,
				"err", err,
			)
			continue
		}
		log.Debug("rag companion search done",
			"stage", "rag.search.companion",
			"category", name,
			"category_id", catID,
			"hits", len(hits),
			"top_score", topScore(hits),
			"search_ms", time.Since(t).Milliseconds(),
		)
		for _, h := range hits {
			id := int(h.ID) //nolint:gosec // dish_id fits in int
			if _, exists := used[id]; exists {
				continue
			}
			used[id] = struct{}{}
			collected = append(collected, id)
			break
		}
	}

	if len(collected) == 0 {
		return nil
	}
	dishes, err := uc.menu.GetDishesByIDs(ctx, collected)
	if err != nil {
		log.Warn("companion load dishes failed", "err", err)
		return nil
	}
	dishesByID := make(map[int]usecasemodels.Dish, len(dishes))
	for _, d := range dishes {
		dishesByID[d.ID] = d
	}
	return buildRetrievedDishes(collected, dishesByID, catName)
}

// withCategoryID возвращает копию base-фильтра с добавленным условием category_id=$id в Must
func withCategoryID(base *qdrant.Filter, categoryID int) *qdrant.Filter {
	out := &qdrant.Filter{
		Must:    append([]qdrant.FilterCondition(nil), base.Must...),
		MustNot: append([]qdrant.FilterCondition(nil), base.MustNot...),
		Should:  append([]qdrant.FilterCondition(nil), base.Should...),
	}
	out.Must = append(out.Must, qdrant.FilterCondition{
		Key:   "category_id",
		Match: &qdrant.FilterMatch{Value: categoryID},
	})
	return out
}

// buildRetrievedDishes мапит id → retrievedDish в порядке ids (id'ы без блюда пропускает)
func buildRetrievedDishes(ids []int, dishesByID map[int]usecasemodels.Dish, catName map[int]string) []retrievedDish {
	out := make([]retrievedDish, 0, len(ids))
	for _, id := range ids {
		d, ok := dishesByID[id]
		if !ok {
			continue
		}
		out = append(out, retrievedDish{
			id:           d.ID,
			name:         d.Name,
			description:  d.Description,
			composition:  d.Composition,
			categoryName: catName[d.CategoryID],
			cuisineRU:    cuisineLabel(string(d.Cuisine)),
			priceMinor:   d.PriceMinor,
		})
	}
	return out
}

// companionIDs возвращает id companion-блюд (для логов/телеметрии)
func companionIDs(comp []retrievedDish) []int {
	out := make([]int, len(comp))
	for i, c := range comp {
		out[i] = c.id
	}
	return out
}

// rerankOrFallback переранжирует через Cohere; при сбое — soft fallback к qdrant-порядку
func (uc *chatUsecase) rerankOrFallback(
	ctx context.Context,
	query string,
	docs []string,
	ids []int,
) []int {
	topN := uc.ragCfg.Search.RerankTopN
	if topN <= 0 || len(docs) == 0 {
		return ids
	}
	results, err := uc.cohere.Rerank(ctx, query, docs, topN)
	if err != nil {
		logger.ForCtx(ctx).Warn("rerank failed, fallback to qdrant order", "err", err)
		return capInts(ids, topN)
	}

	scores := make([]float64, 0, len(results))
	out := make([]int, 0, len(results))
	dropped := 0
	for _, r := range results {
		scores = append(scores, r.Score)
		if r.Score < uc.ragCfg.Search.RerankMinScore {
			dropped++
			continue
		}
		if r.Index < 0 || r.Index >= len(ids) {
			continue
		}
		out = append(out, ids[r.Index])
	}
	logger.ForCtx(ctx).Debug("rag rerank scores",
		"stage", "rag.rerank",
		"scores", scores,
		"min_score", uc.ragCfg.Search.RerankMinScore,
		"dropped_below_min", dropped,
		"kept", len(out),
	)

	// Если порог отсёк всё — лучше отдать LLM хоть какой-то контекст,
	// чем пустой. Берём top-N по cohere-порядку без min-score.
	if len(out) == 0 && len(results) > 0 {
		logger.ForCtx(ctx).Warn("rag rerank all below min, using top-N anyway",
			"stage", "rag.rerank",
			"min_score", uc.ragCfg.Search.RerankMinScore,
			"top_score", scores[0],
		)
		for _, r := range results {
			if r.Index < 0 || r.Index >= len(ids) {
				continue
			}
			out = append(out, ids[r.Index])
			if len(out) >= topN {
				break
			}
		}
	}
	return out
}

// historyResult результат сборки контекста: anchor + recent + priorRecommended
type historyResult struct {
	// anchor хронологически первый user-msg чата (или nil, если чат короткий и
	// он уже попал в recent; либо если у чата нет ни одного user-msg, кроме текущего).
	anchor *repositorymodels.Message
	// recent последние HistoryRecentPairs*2 сообщений в хронологическом порядке
	recent []repositorymodels.Message
	// priorRecommended id блюд из предыдущих assistant-ответов в recent
	priorRecommended []int
}

// loadHistory собирает anchor (первый user-msg чата) и последние N пар сообщений.
// excludeID — id только что добавленного current-user-msg, его никогда не подмешиваем.
func (uc *chatUsecase) loadHistory(
	ctx context.Context,
	chatID uuid.UUID,
	excludeID uuid.UUID,
) (historyResult, error) {
	pairs := uc.ragCfg.Chat.HistoryRecentPairs
	if pairs <= 0 {
		return historyResult{}, nil
	}
	limit := pairs * 2

	raw, _, err := uc.repo.ListMessages(ctx, chatID, limit+1, nil)
	if err != nil {
		return historyResult{}, err
	}
	filtered := make([]repositorymodels.Message, 0, len(raw))
	priorRecommended := make([]int, 0)
	for i := range raw {
		if raw[i].ID == excludeID {
			continue
		}
		filtered = append(filtered, raw[i])
		if raw[i].Role == string(usecasemodels.RoleAssistant) {
			priorRecommended = append(priorRecommended, raw[i].RecommendedDishIDs...)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	reverseMessages(filtered)

	anchor, err := uc.repo.FindFirstUserMessage(ctx, chatID, excludeID)
	if err != nil {
		return historyResult{}, fmt.Errorf("find first user message: %w", err)
	}
	if anchor != nil && len(filtered) > 0 && filtered[0].ID == anchor.ID {
		// anchor уже попал в recent — не дублируем
		anchor = nil
	}

	return historyResult{
		anchor:           anchor,
		recent:           filtered,
		priorRecommended: dedupInts(priorRecommended),
	}, nil
}

// loadChatOwned возвращает чат, если он принадлежит пользователю
func (uc *chatUsecase) loadChatOwned(
	ctx context.Context,
	userID, chatID uuid.UUID,
) (*repositorymodels.Chat, error) {
	c, err := uc.repo.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("find chat: %w", err)
	}
	if c.UserID != userID {
		return nil, chat.ErrChatForbidden
	}
	return c, nil
}

// isStale проверяет, что чат с таким last_message_at пора заменить новым
func (uc *chatUsecase) isStale(lastMessageAt time.Time) bool {
	if uc.chatCfg.AutoNewChatAfter <= 0 {
		return false
	}
	return time.Since(lastMessageAt) > uc.chatCfg.AutoNewChatAfter
}

// buildPrefilter собирает Qdrant-фильтр из аллергенов/диеты пользователя + is_available
func buildPrefilter(u *usecasemodels.User) *qdrant.Filter {
	f := &qdrant.Filter{
		Must: []qdrant.FilterCondition{
			{Key: "is_available", Match: &qdrant.FilterMatch{Value: true}},
		},
	}
	if u != nil {
		for _, a := range u.Allergens {
			f.MustNot = append(f.MustNot, qdrant.FilterCondition{
				Key: "allergens", Match: &qdrant.FilterMatch{Value: a},
			})
		}
		for _, d := range u.Dietary {
			f.Must = append(f.Must, qdrant.FilterCondition{
				Key: "dietary", Match: &qdrant.FilterMatch{Value: d},
			})
		}
	}
	return f
}

// dishToText собирает текст блюда для рерэнкера
func dishToText(d *usecasemodels.Dish) string {
	parts := []string{d.Name}
	if d.Description != "" {
		parts = append(parts, d.Description)
	}
	if d.Composition != "" {
		parts = append(parts, "Состав: "+d.Composition)
	}
	return strings.Join(parts, ". ")
}

// cuisineLabel возвращает русское имя кухни или сам код если маппинга нет
func cuisineLabel(code string) string {
	if v, ok := cuisineRU[code]; ok {
		return v
	}
	return code
}

// parseLLMTail извлекает recommended_dish_ids из JSON-блока в конце ответа и удаляет блок из текста.
// Поддерживает три формата: ```json{...}```, ```{...}```, голый {...} в конце.
func parseLLMTail(raw string) (string, []int) {
	if loc := jsonFencedRe.FindStringSubmatchIndex(raw); loc != nil {
		return extractIDs(raw, loc[0], loc[1], raw[loc[2]:loc[3]])
	}
	if loc := jsonTailRe.FindStringIndex(raw); loc != nil {
		return extractIDs(raw, loc[0], loc[1], raw[loc[0]:loc[1]])
	}
	return strings.TrimSpace(raw), []int{}
}

// extractIDs парсит JSON-payload и возвращает текст без блока
func extractIDs(raw string, blockStart, blockEnd int, jsonRaw string) (string, []int) {
	var payload struct {
		RecommendedDishIDs []int `json:"recommended_dish_ids"`
	}
	_ = json.Unmarshal([]byte(jsonRaw), &payload)
	clean := strings.TrimSpace(raw[:blockStart] + raw[blockEnd:])
	return clean, dedupInts(payload.RecommendedDishIDs)
}

// reverseMessages переворачивает срез сообщений на месте
func reverseMessages(msgs []repositorymodels.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}

// dedupInts возвращает уникальные значения в порядке первого появления
func dedupInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// capInts усекает срез до n элементов
func capInts(in []int, n int) []int {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// topScore возвращает score первого hit-а или 0, если выдача пустая
func topScore(hits []qdrant.SearchHit) float64 {
	if len(hits) == 0 {
		return 0
	}
	return hits[0].Score
}

// previewText обрезает строку до n рун, добавляет «…» если обрезано (для логов)
func previewText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
