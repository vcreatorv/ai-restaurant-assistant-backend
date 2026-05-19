package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
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

// jsonFencedRe ловит JSON-блок recommended_dish_ids, обёрнутый в 1-3 backticks
// с опциональным префиксом "json" (тройной, двойной или одинарный fence).
// GigaChat нередко возвращает блок в одинарных backticks вместо тройных
// (видели на «деталь о блюде» сценариях) — отсюда диапазон {1,3}.
var jsonFencedRe = regexp.MustCompile(
	"(?s)`{1,3}(?:json)?\\s*(\\{[^{}]*\"recommended_dish_ids\"[^{}]*\\})\\s*`{1,3}",
)

// jsonTailRe ловит голый JSON-объект с recommended_dish_ids в самом конце ответа LLM
var jsonTailRe = regexp.MustCompile(`(?s)\{[^{}]*"recommended_dish_ids"[^{}]*\}\s*$`)

// jsonAnywhereRe — последняя попытка: голый JSON-объект с recommended_dish_ids
// где угодно в тексте. Используется, если модель обернула блок чем-то экзотичным
// (вроде HTML-тегов или нестандартных fence), что не подпало под jsonFencedRe.
var jsonAnywhereRe = regexp.MustCompile(`(?s)\{[^{}]*"recommended_dish_ids"[^{}]*\}`)

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

	// 4. Классификация намерения. Отдельный лёгкий LLM-вызов до RAG.
	// Решает, нужен ли вообще RAG и какой промпт подложить ассистенту.
	cls := uc.classify(ctx, content, userID)

	// 5. По интенту собираем prompt для основной LLM. RAG запускаем только когда нужны блюда.
	var rag ragResult
	var prompt []llm.Message
	switch cls.intent {
	case intentOffTopic:
		// Refusal: системный промпт ассистента не используется. Берём refusal из БД,
		// подставляем сообщение пользователя — модель сама сформулирует короткий отказ.
		refusalTpl, perr := uc.prompts.GetActive(ctx, prompts.NameRefusal, userID)
		if perr != nil {
			log.Warn("refusal prompt load failed, using fallback", "err", perr)
			refusalTpl = refusalPromptFallback
		}
		body := strings.ReplaceAll(refusalTpl, "{{user_message}}", content)
		prompt = []llm.Message{{Role: llm.RoleUser, Content: body}}

	case intentChitchat:
		// Chitchat: RAG не нужен (нет вопроса по меню). Используем основной system —
		// в нём уже есть правило короткого ответа на «спасибо/пока».
		systemContent, sysErr := uc.prompts.GetActive(ctx, prompts.NameSystem, userID)
		if sysErr != nil {
			log.Warn("system prompt load failed, using fallback", "err", sysErr)
			systemContent = ""
		}
		prompt = buildPrompt(promptInput{
			systemContent:    systemContent,
			profile:          profile,
			retrieved:        nil, // без меню — buildUserContent напишет «не найдено»
			companions:       nil,
			priorRecommended: hist.priorRecommended,
			anchor:           hist.anchor,
			history:          hist.recent,
			currentUserText:  content,
		})

	default: // intentRecommend, intentClarify
		// Полный pipeline: RAG → system prompt с меню-контекстом.
		var ragErr error
		anchorText := ""
		if hist.anchor != nil {
			anchorText = hist.anchor.Content
		}
		intent, intentErr := uc.buildRetrievalIntent(ctx, cls)
		if intentErr != nil {
			log.Warn("build retrieval intent failed, using empty",
				"stage", "rag.intent", "err", intentErr)
			intent = retrievalIntent{}
		}
		rag, ragErr = uc.runRAG(ctx, content, anchorText, profile, hist.priorRecommended, intent)
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
			"restricted_ids", restrictedIDs(rag.restricted),
			"intent_pairing", intent.pairingDrinkSlug,
			"intent_occasion", intent.occasionSlug,
			"intent_target_categories", intent.targetCategoryIDs,
			"intent_price", intent.priceIntent,
			"intent_meal_structure", intent.mealStructure,
		)
		systemContent, sysErr := uc.prompts.GetActive(ctx, prompts.NameSystem, userID)
		if sysErr != nil {
			log.Warn("system prompt load failed, using fallback", "err", sysErr)
			systemContent = ""
		}
		prompt = buildPrompt(promptInput{
			systemContent:      systemContent,
			profile:            profile,
			retrieved:          rag.main,
			companions:         rag.companions,
			restricted:         rag.restricted,
			priorRecommended:   hist.priorRecommended,
			anchor:             hist.anchor,
			history:            hist.recent,
			currentUserText:    content,
			targetCategoryName: singleTargetCategoryName(cls.targetCategorySlugs),
		})
	}

	// 6. Готовим metadata для клиента и assistant-сообщения.
	assistantMessageID := uc.uuid.New()
	if cb.OnMeta != nil {
		if err := cb.OnMeta(chat.MetaEvent{
			MessageID:          assistantMessageID,
			RecommendedDishIDs: rag.rerankedIDs, // пусто для chitchat/off_topic
		}); err != nil {
			return fmt.Errorf("on meta: %w", err)
		}
	}
	llmStart := time.Now()
	log.Debug("chat llm prompt assembled",
		"stage", "llm.prompt",
		"messages_count", len(prompt),
		"main_dishes", len(rag.main),
		"companions", len(rag.companions),
	)
	var fullText strings.Builder
	// SessionID = chat_id: для GigaChat это X-Session-ID, кэширующий префикс
	// контекста между запросами одного чата (меньше токенов на длинных диалогах).
	// Для NVIDIA/OpenRouter заголовок игнорируется — безопасно ставить всегда.
	usage, llmErr := uc.llm.ChatStream(ctx, llm.ChatRequest{
		Messages:  prompt,
		SessionID: chatID.String(),
	}, func(delta string) error {
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
	// Сырой ответ логируем целиком на DEBUG — нужно для разбора кейсов,
	// когда LLM не вернула финальный JSON-блок: только так видно, был ли он
	// вообще в ответе модели или это наш parser промахнулся.
	raw := fullText.String()
	log.Debug("chat llm raw response",
		"stage", "llm.raw",
		"text", raw,
		"text_len", len(raw),
	)
	cleanText, llmRecommended := parseLLMTail(raw)
	log.Debug("chat llm parsed",
		"stage", "llm.parse",
		"clean_text_preview", previewText(cleanText, 120),
		"llm_recommended", llmRecommended,
	)

	// Fallback: некоторые модели (GigaChat и старшие, не GPT-4-класса) плохо
	// следуют инструкции «в конце добавь JSON-блок». Если массив пустой —
	// пытаемся восстановить id по упоминаниям блюд в тексте: матчим имена в
	// **double-stars** с known-блюдами из подаваемого RAG-контекста.
	if len(llmRecommended) == 0 {
		llmRecommended = recoverIDsFromText(cleanText, rag.main, rag.companions)
		if len(llmRecommended) > 0 {
			log.Debug("chat llm recovered ids from text",
				"stage", "llm.parse.fallback",
				"recovered", llmRecommended,
			)
		}
	}

	// recommended_dish_ids в API/БД отражает то, что LLM реально упомянула в ответе.
	// От рерэнкера в meta остаётся отдельное поле reranked_ids — для аналитики.
	finalRecommended := llmRecommended
	if finalRecommended == nil {
		finalRecommended = []int{}
	}

	// 8. Assistant-message в БД с полной телеметрией.
	assistantMeta := map[string]any{
		"latency_ms":              time.Since(start).Milliseconds(),
		"tokens_in":               usage.PromptTokens,
		"tokens_out":              usage.CompletionTokens,
		"finish_reason":           usage.FinishReason,
		"model":                   usage.Model,
		"retrieved_ids":           rag.retrievedIDs,
		"reranked_ids":            rag.rerankedIDs,
		"companion_ids":           companionIDs(rag.companions),
		"intent":                  string(cls.intent),
		"classifier_pairing":      cls.pairingDrink,
		"classifier_occasion":     cls.occasion,
		"classifier_categories":   cls.targetCategorySlugs,
		"classifier_price":        cls.priceIntent,
		"classifier_meal":         cls.mealStructure,
		"classifier_latency_ms":   cls.latency.Milliseconds(),
		"classifier_failed":       cls.failed,
		"classifier_raw_response": cls.rawResponse,
	}
	if cls.failureReason != "" {
		assistantMeta["classifier_failure"] = cls.failureReason
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

	// 9. Финальный done. Передаём актуальный recommended_dish_ids — это нужно фронту,
	// чтобы он показал именно реально названные ассистентом блюда, а не все RAG-кандидаты
	// из meta-события (которые он получил до начала стрима).
	if cb.OnDone != nil {
		if err := cb.OnDone(chat.DoneEvent{
			LatencyMS:          time.Since(start).Milliseconds(),
			TokensIn:           usage.PromptTokens,
			TokensOut:          usage.CompletionTokens,
			Model:              usage.Model,
			RecommendedDishIDs: finalRecommended,
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
	// restricted блюда, семантически близкие к запросу, но отрезанные аллерген/диета
	// фильтром гостя. Передаются в prompt отдельным блоком — LLM упоминает их,
	// только если гость прямо спросил про конкретное блюдо. См. computeRestricted.
	restricted []retrievedDish
	// retrievedIDs все id, поднятые primary-search'ем (для аналитики)
	retrievedIDs []int
	// rerankedIDs id после rerank, в порядке убывания релевантности
	rerankedIDs []int
}

// runRAG выполняет embed → primary search/rerank → companion searches → load.
//
// Базово embed и rerank работают по текущему сообщению — иначе разнотемные предыдущие
// вопросы смазывают вектор и rerank теряет фокус. Контекст диалога LLM получает
// отдельно через массив messages.
//
// Исключение: если в текущем сообщении есть указательные/референциальные слова
// («эти», «к ним», «к этому», «такой же», «ещё»), сами по себе они не несут
// смысла блюда — без склейки с anchor (первый user-msg чата) retrieval вытянет
// универсальные позиции. В этом случае склеиваем `anchor + " " + currentMessage`
// для embed/rerank, чтобы фокус остался на исходной теме («гарнир к рыбе»,
// а не просто «гарнир»).
//
// priorRecommendedIDs — id блюд, уже названных в этом чате; они исключаются из Qdrant
// must_not по dish_id, чтобы новые сообщения получали в RAG-контексте свежие позиции,
// а не повторно те же «мороженое»/«картофель фри», которые тяжёлый эмбеддинг обычно
// держит в верхушке top-k.
func (uc *chatUsecase) runRAG(
	ctx context.Context,
	currentMessage string,
	anchorMessage string,
	profile *usecasemodels.User,
	priorRecommendedIDs []int,
	intent retrievalIntent,
) (ragResult, error) {
	log := logger.ForCtx(ctx)

	query := effectiveRAGQuery(currentMessage, anchorMessage)
	log.Debug("rag query resolved",
		"stage", "rag.query",
		"resolved_len", len(query),
		"used_anchor", query != currentMessage,
	)

	t0 := time.Now()
	embeds, err := uc.cohere.Embed(ctx, []string{query}, rag.CohereInputQuery)
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

	// Поток A: vector-search с базовым prefilter (user/prior) + intent-hard-фильтры
	// (target_category, price) — НО без pairing/occasion тегов. Это «семантика +
	// hard-constraints»; страхует от провалов pairing-разметки.
	prefilter := buildPrefilter(profile, priorRecommendedIDs)
	applyIntentFilters(prefilter, intent, false)
	hitsA, err := uc.qdrant.Search(ctx, embed, prefilter, uc.ragCfg.Search.TopK, true)
	if err != nil {
		return ragResult{}, fmt.Errorf("%w: search: %s", chat.ErrUpstreamFailure, err.Error())
	}
	log.Debug("rag primary search done (stream A: semantic)",
		"stage", "rag.search.primary",
		"hits", len(hitsA),
		"top_score", topScore(hitsA),
		"target_category_ids", intent.targetCategoryIDs,
		"price_intent", intent.priceIntent,
		"meal_structure", intent.mealStructure,
	)

	// Поток B: vector-search внутри pairing/occasion подмножества (теги в must).
	// Запускаем ТОЛЬКО при hybrid-режиме: если есть pairing/occasion И нет
	// hard-category-filter (category уже сузила всё). См. retrievalIntent.hasHybrid.
	hits := hitsA
	if intent.hasHybrid() {
		filterB := buildPrefilter(profile, priorRecommendedIDs)
		applyIntentFilters(filterB, intent, true)
		hitsB, errB := uc.qdrant.Search(ctx, embed, filterB, uc.ragCfg.Search.TopK, true)
		switch {
		case errB != nil:
			log.Warn("rag search (stream B) failed, falling back to single-stream",
				"stage", "rag.search.pairing", "err", errB,
				"pairing", intent.pairingDrinkSlug, "occasion", intent.occasionSlug)
		case len(hitsB) < rrfMinStreamHits:
			log.Warn("rag tag stream too thin, ignoring",
				"stage", "rag.search.pairing",
				"pairing", intent.pairingDrinkSlug, "occasion", intent.occasionSlug,
				"hits", len(hitsB), "min", rrfMinStreamHits)
		default:
			hits = mergeHitsRRF(hitsA, hitsB, uc.ragCfg.Search.TopK)
			log.Info("rag hybrid retrieval (RRF)",
				"stage", "rag.search.hybrid",
				"pairing", intent.pairingDrinkSlug, "occasion", intent.occasionSlug,
				"hits_a", len(hitsA), "hits_b", len(hitsB),
				"merged", len(hits), "top_score_a", topScore(hitsA), "top_score_b", topScore(hitsB),
			)
		}
	}

	categories, err := uc.menu.ListCategories(ctx)
	if err != nil {
		return ragResult{}, fmt.Errorf("list categories: %w", err)
	}
	catName := make(map[int]string, len(categories))
	var mainCats, companionCats []usecasemodels.Category
	for _, c := range categories {
		catName[c.ID] = c.Name
		switch c.Role {
		case usecasemodels.CategoryRoleMain:
			mainCats = append(mainCats, c)
		case usecasemodels.CategoryRoleCompanion:
			companionCats = append(companionCats, c)
		}
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

	// Category-restricted mode: гость явно назвал одну или несколько категорий.
	// Primary уже отдал блюда только из них (фильтр Must.category_id Value или Any).
	// Rerank/diversify/companions в этом режиме мешают:
	//  - rerank внутри одной узкой категории даёт scores ~0.001-0.013 (все близки),
	//    порог 0.01 выкосит 7 из 8 → LLM остаётся с одним блюдом и галлюцинирует.
	//  - diversify пробует добавить top-1 из других main-категорий, но фильтр
	//    AND по category_id всегда даёт 0 (блюдо не может быть в двух категориях).
	//  - companions добавили бы соус/гарнир/десерт/напиток — гость о них не просил.
	//
	// На multi-target («пицца с бургером», len>1) тот же подход тоже верный:
	// primary с match.Any вернул блюда из ОБЕИХ категорий, дальше пусть LLM выбирает.
	//
	// Отдаём все primary-hits в main как есть, без rerank/diversify/companions.
	if len(intent.targetCategoryIDs) > 0 {
		main := buildRetrievedDishes(retrievedIDs, dishesByID, catName)
		mainSet := make(map[int]struct{}, len(retrievedIDs))
		for _, id := range retrievedIDs {
			mainSet[id] = struct{}{}
		}
		restricted := uc.runRestricted(ctx, embed, profile, mainSet, priorRecommendedIDs, catName)
		log.Info("rag category-restricted mode",
			"stage", "rag.cross_sell",
			"target_category_ids", intent.targetCategoryIDs,
			"main_count", len(main),
		)
		return ragResult{
			main:         main,
			restricted:   restricted,
			retrievedIDs: retrievedIDs,
			rerankedIDs:  retrievedIDs,
		}, nil
	}

	rerankInput := make([]string, 0, len(hits))
	rerankIDs := make([]int, 0, len(hits))
	for _, h := range hits {
		d, ok := dishesByID[int(h.ID)] //nolint:gosec // dish_id fits in int
		if !ok {
			continue
		}
		rerankInput = append(rerankInput, dishToText(&d, catName[d.CategoryID]))
		rerankIDs = append(rerankIDs, d.ID)
	}

	t2 := time.Now()
	rerankedIDs := uc.rerankOrFallback(ctx, query, rerankInput, rerankIDs)
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
		mainCatSet[d.categoryID] = struct{}{}
	}

	// Диверсификация main по категориям (если в reranked top-N покрыто мало
	// разных main-категорий — добавляем top-1 из непокрытых).
	//
	// Для clarify-запросов («что входит в X», «а напиток к этому?») diversify
	// пропускаем: гость не просил разнообразия, а уточнил что-то узкое. Без
	// этого guard primary-search может вернуть 8 напитков → diversify добавит
	// Пастрами salad / Том-ям / Ризотто «для main-категорий», и LLM соблазнится
	// рекомендовать салат вместо напитка (наблюдали в логе).
	//
	// При meal_structure ∈ {full_dinner, full_lunch} включается forceFullCoverage:
	// пороги обнуляются и каркас (закуска/салат → горячее → десерт) добивается
	// принудительно. Без этого «сытный ужин» возвращает 3 салата (lexical overlap
	// «сытный салат-бургер»), supplier-категории не добираются — cosine score у
	// супов/стейков относительно «сытный ужин» обычно ниже MainDiversifyMinScore=0.4.
	if !intent.isClarify {
		diversified := uc.runMainDiversification(
			ctx, embed, prefilter, mainSet, mainCatSet, mainCats, catName,
			intent.hasFullMealStructure(),
		)
		for _, d := range diversified {
			main = append(main, d)
			mainSet[d.id] = struct{}{}
			mainCatSet[d.categoryID] = struct{}{}
		}
	}

	// Companions добавляем только когда основной поток ≠ узкий clarify-вопрос.
	// На «расскажи про X» / «а напиток к этому?» лишние блюда из других
	// companion-категорий — шум.
	var companions []retrievedDish
	if !intent.isClarify {
		companions = uc.runCompanions(ctx, embed, prefilter, mainSet, mainCatSet, companionCats, catName)
	}

	// Restricted: блюда, попавшие бы в выдачу по семантике, но отрезанные фильтром
	// профиля гостя (аллергены/диета). Идут отдельным блоком в prompt — LLM может
	// их упомянуть только если гость прямо спросил про конкретное блюдо.
	// См. computeRestricted и computeRestrictionReason.
	restricted := uc.runRestricted(ctx, embed, profile, mainSet, priorRecommendedIDs, catName)

	return ragResult{
		main:         main,
		companions:   companions,
		restricted:   restricted,
		retrievedIDs: retrievedIDs,
		rerankedIDs:  rerankedIDs,
	}, nil
}

// runRestricted ищет блюда, отрезанные аллерген/диета-фильтром гостя.
//
// Алгоритм: ещё один Qdrant.Search тем же вектором запроса, но БЕЗ
// аллерген/диета условий (см. buildPrefilterLoose); сравниваем с уже
// найденными в primary search (mainSet) — разница и есть «есть в меню, но не
// для гостя». Берём top-N с заполненной причиной (computeRestrictionReason).
//
// Лимит — restrictedMaxItems, чтобы не раздувать prompt-context.
func (uc *chatUsecase) runRestricted(
	ctx context.Context,
	embed []float32,
	profile *usecasemodels.User,
	mainSet map[int]struct{},
	priorRecommendedIDs []int,
	catName map[int]string,
) []retrievedDish {
	log := logger.ForCtx(ctx)
	// Нет ограничений у гостя — пропускаем целый стейдж, экономим Qdrant-вызов.
	if profile == nil || (len(profile.Allergens) == 0 && len(profile.Dietary) == 0) {
		return nil
	}
	looseFilter := buildPrefilterLoose(priorRecommendedIDs)
	looseHits, err := uc.qdrant.Search(ctx, embed, looseFilter, uc.ragCfg.Search.TopK, false)
	if err != nil {
		log.Warn("rag restricted search failed (non-fatal)",
			"stage", "rag.search.restricted", "err", err)
		return nil
	}

	candidateIDs := make([]int, 0, restrictedMaxItems)
	for _, h := range looseHits {
		id := int(h.ID) //nolint:gosec // dish_id fits in int
		if _, inMain := mainSet[id]; inMain {
			continue
		}
		candidateIDs = append(candidateIDs, id)
		if len(candidateIDs) >= restrictedMaxItems {
			break
		}
	}
	if len(candidateIDs) == 0 {
		log.Debug("rag restricted: no diff between loose and primary",
			"stage", "rag.search.restricted",
			"loose_hits", len(looseHits))
		return nil
	}

	dishes, err := uc.menu.GetDishesByIDs(ctx, candidateIDs)
	if err != nil {
		log.Warn("rag restricted load dishes failed",
			"stage", "rag.search.restricted", "err", err)
		return nil
	}
	dishesByID := make(map[int]usecasemodels.Dish, len(dishes))
	for _, d := range dishes {
		dishesByID[d.ID] = d
	}

	out := make([]retrievedDish, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		d, ok := dishesByID[id]
		if !ok {
			continue
		}
		reason := computeRestrictionReason(retrievedDishRaw{
			allergens: d.Allergens,
			dietary:   d.Dietary,
		}, profile)
		if reason == "" {
			// Блюдо в diff, но конфликта по аллерген/диета не нашлось — значит,
			// его отрезал какой-то другой must (например, dish_id в priorIDs).
			// Сюда не дойдёт по построению, но защищаемся.
			continue
		}
		out = append(out, retrievedDish{
			id:                d.ID,
			name:              d.Name,
			categoryID:        d.CategoryID,
			categoryName:      catName[d.CategoryID],
			restrictionReason: reason,
		})
	}
	log.Info("rag restricted",
		"stage", "rag.search.restricted",
		"count", len(out),
		"ids", restrictedIDs(out),
	)
	return out
}

// restrictedMaxItems лимит блюд в restricted-блоке (чтобы не раздувать prompt).
// 8 — это покрывает «у гостя аллергия на рыбу, он спросил рыбу» и видны все
// морепродукты с причинами; больше LLM уже не использует осмысленно.
const restrictedMaxItems = 8

// restrictedIDs выдёргивает id из retrievedDish'ей — для лога.
func restrictedIDs(rs []retrievedDish) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.id
	}
	return out
}

// runMainDiversification на широких запросах добавляет в main top-1 из тех
// main-категорий (categories с role='main'), которые не покрыты reranked top-N.
// Срабатывает только если: (а) в main < cfg.MainMinCategories разных main-категорий,
// (б) score top-1 >= cfg.MainDiversifyMinScore (на узком запросе нерелевантное не подсосётся).
// Лимит cfg.MainMaxAdded.
//
// При forceFullCoverage=true пороги обнуляются: minRequired = len(mainCats),
// minScore=0, maxAdded=len(mainCats). Это нужно при meal_structure="full_dinner"/
// "full_lunch": гость хочет полную трапезу (закуска→горячее→десерт), и порог
// 0.4 ронял бы supplier-категории из-за низкого cosine score у запроса вроде
// «сытный ужин», который лексически близок только к одной-двум категориям.
func (uc *chatUsecase) runMainDiversification(
	ctx context.Context,
	embed []float32,
	base *qdrant.Filter,
	mainSet map[int]struct{},
	mainCatSet map[int]struct{},
	mainCats []usecasemodels.Category,
	catName map[int]string,
	forceFullCoverage bool,
) []retrievedDish {
	log := logger.ForCtx(ctx)
	minCats := uc.ragCfg.Chat.MainMinCategories
	maxAdded := uc.ragCfg.Chat.MainMaxAdded
	minScore := uc.ragCfg.Chat.MainDiversifyMinScore
	if forceFullCoverage {
		minCats = len(mainCats)
		maxAdded = len(mainCats)
		minScore = 0
	}
	if len(mainCats) == 0 || maxAdded <= 0 {
		return nil
	}

	// Считаем покрытые main-категории (только те, у которых role='main').
	covered := 0
	mainCatIDs := make(map[int]struct{}, len(mainCats))
	for _, c := range mainCats {
		mainCatIDs[c.ID] = struct{}{}
	}
	for catID := range mainCatSet {
		if _, ok := mainCatIDs[catID]; ok {
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
	for _, c := range mainCats {
		if len(collected) >= maxAdded {
			break
		}
		if _, alreadyCovered := mainCatSet[c.ID]; alreadyCovered {
			continue
		}
		filter := withCategoryID(base, c.ID)

		t := time.Now()
		hits, err := uc.qdrant.Search(ctx, embed, filter, 1, false)
		if err != nil {
			log.Warn("rag diversify search failed",
				"stage", "rag.diversify",
				"category", c.Name,
				"category_id", c.ID,
				"err", err,
			)
			continue
		}
		if len(hits) == 0 {
			continue
		}
		log.Debug("rag diversify search done",
			"stage", "rag.diversify",
			"category", c.Name,
			"category_id", c.ID,
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

// runCompanions для каждой companion-категории (categories с role='companion') делает
// Qdrant.Search с pre-filter (профиль + is_available + category_id) и берёт top-1, дедупит с main.
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
	companionCats []usecasemodels.Category,
	catName map[int]string,
) []retrievedDish {
	log := logger.ForCtx(ctx)
	if len(companionCats) == 0 {
		return nil
	}

	used := make(map[int]struct{}, len(mainSet)+len(companionCats))
	for id := range mainSet {
		used[id] = struct{}{}
	}

	collected := make([]int, 0, len(companionCats))
	for _, c := range companionCats {
		if _, exists := mainCatSet[c.ID]; exists {
			log.Debug("rag companion skipped (category already in main)",
				"stage", "rag.search.companion",
				"category", c.Name,
				"category_id", c.ID,
			)
			continue
		}
		filter := withCategoryID(base, c.ID)

		t := time.Now()
		hits, err := uc.qdrant.Search(ctx, embed, filter, 5, false)
		if err != nil {
			log.Warn("rag companion search failed",
				"stage", "rag.search.companion",
				"category", c.Name,
				"category_id", c.ID,
				"err", err,
			)
			continue
		}
		log.Debug("rag companion search done",
			"stage", "rag.search.companion",
			"category", c.Name,
			"category_id", c.ID,
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
			categoryID:   d.CategoryID,
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

// referenceWordRe детектит указательные/референциальные слова в текущем сообщении гостя.
// Если они найдены, retrieval склеивает anchor + current, чтобы не потерять тему диалога:
// например, «а что на гарнир к этим блюдам?» без склейки даёт универсальные гарниры,
// а со склейкой anchor=«хочу что-то из морских продуктов» — гарниры именно к морепродуктам.
//
// Паттерн нарочно широкий и нестрогий: лучше иногда дать anchor, чем потерять контекст.
var referenceWordRe = regexp.MustCompile(`(?i)\b(эт(о|и|а|их|им|ими|ого|ому|ой|ом)|так(ой|ая|ое|ие|ому|ой)|так\s+же|так-то|тот|та|те|их|им|ими|ему|ей|ему|нему|ним|ним|ним же|ним-то|ним|него|неё|них|нему|нему же|ещё|еще)\b`)

// effectiveRAGQuery определяет, что подавать в Cohere embed и rerank.
//
// По умолчанию это просто currentMessage. Но если в currentMessage есть указательные слова
// («эти», «к ним», «такой же», «ещё») и в чате есть anchor — мы склеиваем «anchor . current»,
// чтобы retrieval сохранил тему первого вопроса.
func effectiveRAGQuery(currentMessage, anchorMessage string) string {
	if strings.TrimSpace(anchorMessage) == "" {
		return currentMessage
	}
	if !referenceWordRe.MatchString(currentMessage) {
		return currentMessage
	}
	return anchorMessage + ". " + currentMessage
}

// buildPrefilter собирает Qdrant-фильтр: is_available + hard-фильтры профиля
// (аллергены/диета) + список priorRecommended dish_id, которые надо исключить
// из выдачи (чтобы LLM не получал в контексте уже названные блюда — иначе
// «десерт = мороженое» залипает между сообщениями одного чата).
func buildPrefilter(u *usecasemodels.User, priorRecommendedIDs []int) *qdrant.Filter {
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
	if len(priorRecommendedIDs) > 0 {
		anyVals := make([]any, len(priorRecommendedIDs))
		for i, id := range priorRecommendedIDs {
			anyVals[i] = id
		}
		f.MustNot = append(f.MustNot, qdrant.FilterCondition{
			Key:   "dish_id",
			Match: &qdrant.FilterMatch{Any: anyVals},
		})
	}
	return f
}

// retrievalIntent — структурированное намерение, извлечённое classifier'ом
// и используемое для построения фильтров и hybrid-выбора в runRAG.
//
// Поля независимы и комбинируются:
//   - targetCategoryIDs — HARD-фильтр Stream A. При len==1 это cross-sell вроде
//     «гарнир к стейку» (внутри одной категории по семантике). При len>1 — multi-
//     category запрос вроде «пицца с бургером» (фильтр category_id IN (X,Y) через
//     match.Any). При len>0 hybrid отключается.
//   - priceIntent — range-фильтр по price_minor (cheap/premium).
//   - pairingDrinkSlug / occasionSlug — теги для Stream B + RRF.
//     Когда хотя бы один задан и targetCategoryIDs пуст → hybrid retrieval.
//   - mealStructure — паттерн трапезы (см. validMealStructures). full_dinner/full_lunch
//     включают принудительный каркасный сбор в runMainDiversification (без порога
//     MainDiversifyMinScore). fastfood_set разворачивается в buildRetrievalIntent
//     в targetCategoryIDs из fastfoodCategorySlugs.
type retrievalIntent struct {
	pairingDrinkSlug  string
	occasionSlug      string
	targetCategoryIDs []int
	priceIntent       string
	mealStructure     string
	// isClarify — это follow-up-уточнение («что входит в X», «а напиток к этому?»).
	// Запросы такого типа всегда узкие. Diversify здесь — шум: гость не просил
	// «разнообразия», а спросил про конкретное. Поэтому при isClarify=true мы
	// пропускаем runMainDiversification (см. runRAG).
	isClarify bool
}

// buildRetrievalIntent резолвит classifier-результат в retrievalIntent:
//   - pairingDrink («white_wine») → pair_white_wine (готовый slug в pairing_tags)
//   - occasion («date») → occasion_date
//   - targetCategorySlugs (["side", "burger"]) → имена категорий → category_id через ListCategories
//   - priceIntent — пробрасывается как есть
//   - mealStructure — пробрасывается как есть; "fastfood_set" разворачивается в
//     targetCategoryIDs через fastfoodCategorySlugs, если classifier сам не дал
//     явный список категорий.
//
// Если не удалось загрузить категории (PG недоступен) — возвращает intent без
// targetCategoryIDs. Остальные поля независимы от БД и заполняются в любом случае.
func (uc *chatUsecase) buildRetrievalIntent(
	ctx context.Context,
	cls classifyResult,
) (retrievalIntent, error) {
	intent := retrievalIntent{
		pairingDrinkSlug: pairingDrinkToSlug(cls.pairingDrink),
		occasionSlug:     occasionToSlug(cls.occasion),
		priceIntent:      cls.priceIntent,
		mealStructure:    cls.mealStructure,
		isClarify:        cls.intent == intentClarify,
	}

	// Резолв slug → имя категории → category_id. Для fastfood_set без явного
	// списка категорий разворачиваем дефолтный set (бургеры/гарниры/закуски/напитки).
	slugs := cls.targetCategorySlugs
	if len(slugs) == 0 && cls.mealStructure == "fastfood_set" {
		slugs = fastfoodCategorySlugs
	}
	if len(slugs) == 0 {
		return intent, nil
	}

	categories, err := uc.menu.ListCategories(ctx)
	if err != nil {
		return intent, fmt.Errorf("list categories for target_categories: %w", err)
	}
	nameToID := make(map[string]int, len(categories))
	for _, c := range categories {
		nameToID[c.Name] = c.ID
	}
	for _, slug := range slugs {
		name := targetCategoryToName(slug)
		if name == "" {
			continue
		}
		id, ok := nameToID[name]
		if !ok {
			continue
		}
		intent.targetCategoryIDs = append(intent.targetCategoryIDs, id)
	}
	return intent, nil
}

// hasHybrid возвращает true, если есть pairing/occasion И при этом нет hard-фильтра
// по category. В таком случае runRAG запускает Stream B + RRF. Если category
// задана — она уже сужает результат, hybrid избыточен и может смазать ранжирование.
func (i retrievalIntent) hasHybrid() bool {
	if len(i.targetCategoryIDs) > 0 {
		return false
	}
	return i.pairingDrinkSlug != "" || i.occasionSlug != ""
}

// hasFullMealStructure true, если гость хочет полноценный приём пищи (ужин/обед).
// Включает каркасный сбор в runMainDiversification без порога MainDiversifyMinScore —
// иначе на «сытный ужин» supplied embed-семантикой 3 салата (лексический оверлап
// «сытный») остаются без супа/горячего/десерта.
func (i retrievalIntent) hasFullMealStructure() bool {
	return i.mealStructure == "full_dinner" || i.mealStructure == "full_lunch"
}

// hasTagFilter true, если в intent есть хоть один pairing/occasion тег.
// Используется только применительно к Stream B (см. applyIntentFilters).
func (i retrievalIntent) hasTagFilter() bool {
	return i.pairingDrinkSlug != "" || i.occasionSlug != ""
}

// cheapPriceMaxMinor — верхняя граница price_minor для intent «cheap».
// 1500 ₽ — эмпирически между «закусками/гарнирами/напитками» (300-1500 ₽)
// и основными горячими (1500-5000 ₽). Можно подкрутить позже.
const cheapPriceMaxMinor = 150000

// premiumPriceMinMinor — нижняя граница price_minor для intent «premium».
// 2500 ₽ — выше среднего main, отсекает гарниры/закуски/лимонады.
const premiumPriceMinMinor = 250000

// priceRangeFor возвращает Qdrant range-фильтр для price_minor или nil,
// если price_intent не задан.
func priceRangeFor(priceIntent string) *qdrant.FilterRange {
	switch priceIntent {
	case "cheap":
		v := float64(cheapPriceMaxMinor)
		return &qdrant.FilterRange{LTE: &v}
	case "premium":
		v := float64(premiumPriceMinMinor)
		return &qdrant.FilterRange{GTE: &v}
	}
	return nil
}

// applyIntentFilters добавляет в готовый фильтр условия из intent.
// applyTags=true означает «добавь pairing/occasion-теги» — это нужно только
// для Stream B (vector-поиск внутри тегированного подмножества). category_id
// и price range применяются всегда — это hard-фильтры, релевантные обоим потокам.
func applyIntentFilters(f *qdrant.Filter, intent retrievalIntent, applyTags bool) {
	switch len(intent.targetCategoryIDs) {
	case 0:
		// без category-фильтра
	case 1:
		f.Must = append(f.Must, qdrant.FilterCondition{
			Key:   "category_id",
			Match: &qdrant.FilterMatch{Value: intent.targetCategoryIDs[0]},
		})
	default:
		// Qdrant match.Any эквивалентен SQL IN: блюдо проходит фильтр, если его
		// category_id совпал с любым из элементов. Используется для multi-target
		// запросов вроде «пицца с бургером».
		anyVals := make([]any, len(intent.targetCategoryIDs))
		for i, id := range intent.targetCategoryIDs {
			anyVals[i] = id
		}
		f.Must = append(f.Must, qdrant.FilterCondition{
			Key:   "category_id",
			Match: &qdrant.FilterMatch{Any: anyVals},
		})
	}
	if r := priceRangeFor(intent.priceIntent); r != nil {
		f.Must = append(f.Must, qdrant.FilterCondition{Key: "price_minor", Range: r})
	}
	if applyTags {
		if intent.pairingDrinkSlug != "" {
			f.Must = append(f.Must, qdrant.FilterCondition{
				Key:   "pairing_tags",
				Match: &qdrant.FilterMatch{Value: intent.pairingDrinkSlug},
			})
		}
		if intent.occasionSlug != "" {
			f.Must = append(f.Must, qdrant.FilterCondition{
				Key:   "pairing_tags",
				Match: &qdrant.FilterMatch{Value: intent.occasionSlug},
			})
		}
	}
}

// rrfMinStreamHits — минимальный размер потока B (pairing-search), при котором
// мы доверяем hybrid retrieval. Меньшее значит, что наша pairing-разметка слишком
// разрежена для этого запроса; чище отвалиться на single-stream A, чем подтащить
// 1-2 случайных блюда с весами RRF и сломать ранжирование.
const rrfMinStreamHits = 3

// rrfK — константа сглаживания в Reciprocal Rank Fusion: score = 1/(k + rank).
// 60 — каноническое значение из оригинальной статьи Cormack et al. (2009);
// смещает влияние одного потока для top-ranked документов и сглаживает разницу
// между потоками с разным числом hits.
const rrfK = 60

// mergeHitsRRF сливает два списка кандидатов через Reciprocal Rank Fusion.
// Каждый hit получает score = 1/(rrfK + rank_in_stream + 1); если hit попал в оба
// потока — score'ы суммируются. Результат сортируется по убыванию суммарного score,
// возвращается topN. Если topN ≤ 0 — без ограничения.
//
// Поле SearchHit.Score в merged-выдаче перетирается на rrf-score (для дальнейших
// логов и rerank). Payload берётся из A (если hit есть в обоих), иначе из B.
func mergeHitsRRF(a, b []qdrant.SearchHit, topN int) []qdrant.SearchHit {
	type acc struct {
		score   float64
		fromA   *qdrant.SearchHit
		fromB   *qdrant.SearchHit
		firstAt int // позиция в первом потоке, где встретился — для стабильной сортировки
	}
	bucket := make(map[uint64]*acc, len(a)+len(b))
	add := func(stream []qdrant.SearchHit, isA bool) {
		for rank, h := range stream {
			entry, ok := bucket[h.ID]
			if !ok {
				entry = &acc{firstAt: rank}
				bucket[h.ID] = entry
			}
			entry.score += 1.0 / float64(rrfK+rank+1)
			if isA {
				hh := h
				entry.fromA = &hh
			} else {
				hh := h
				entry.fromB = &hh
			}
		}
	}
	add(a, true)
	add(b, false)

	out := make([]qdrant.SearchHit, 0, len(bucket))
	for _, e := range bucket {
		base := e.fromA
		if base == nil {
			base = e.fromB
		}
		out = append(out, qdrant.SearchHit{
			ID:      base.ID,
			Score:   e.score,
			Payload: base.Payload,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out
}

// buildPrefilterLoose собирает фильтр БЕЗ ограничений профиля гостя
// (аллергены/диета снимаются), но с is_available и priorRecommended.
// Используется в runRAG для расчёта restricted-set: блюда, которые есть в меню
// и попали бы в выдачу по семантике, но были отрезаны фильтром гостя.
func buildPrefilterLoose(priorRecommendedIDs []int) *qdrant.Filter {
	f := &qdrant.Filter{
		Must: []qdrant.FilterCondition{
			{Key: "is_available", Match: &qdrant.FilterMatch{Value: true}},
		},
	}
	if len(priorRecommendedIDs) > 0 {
		anyVals := make([]any, len(priorRecommendedIDs))
		for i, id := range priorRecommendedIDs {
			anyVals[i] = id
		}
		f.MustNot = append(f.MustNot, qdrant.FilterCondition{
			Key:   "dish_id",
			Match: &qdrant.FilterMatch{Any: anyVals},
		})
	}
	return f
}

// allergenRU и dietaryRU — короткие русские формулировки для restriction-reason.
// Берутся в винительном для allergens («содержит рыбу») и в номинативе для dietary
// («не вегетарианское»). Неизвестный код фоллбэчится на сам код.
//
// shellfish переводим как «морепродукты» — общий термин, который для гостя
// покрывает и креветки (ракообразные), и устрицы/мидии (моллюски). Если
// захотим точнее — нужно разделить seed-код на crustaceans / mollusks
// (отдельный шаг, требует переразметки данных).
var allergenRU = map[string]string{
	"fish":      "рыбу",
	"shellfish": "морепродукты",
	"gluten":    "глютен",
	"nuts":      "орехи",
	"eggs":      "яйца",
	"dairy":     "молочное",
	"lactose":   "лактозу",
	"soy":       "сою",
	"sesame":    "кунжут",
	"mustard":   "горчицу",
	"celery":    "сельдерей",
	"peanuts":   "арахис",
}

var dietaryRU = map[string]string{
	"vegan":        "веганское",
	"vegetarian":   "вегетарианское",
	"gluten_free":  "без глютена",
	"lactose_free": "без лактозы",
	"halal":        "халяль",
	"kosher":       "кошер",
}

// computeRestrictionReason для блюда d и профиля гостя возвращает короткую
// человеко-читаемую причину, почему блюдо не подходит. Пустая строка означает,
// что конфликта нет (на всякий случай — теоретически такого быть не должно,
// если блюдо попало в restricted-set).
func computeRestrictionReason(d retrievedDishRaw, profile *usecasemodels.User) string {
	if profile == nil {
		return ""
	}
	dishAllergens := make(map[string]struct{}, len(d.allergens))
	for _, a := range d.allergens {
		dishAllergens[a] = struct{}{}
	}
	dishDietary := make(map[string]struct{}, len(d.dietary))
	for _, x := range d.dietary {
		dishDietary[x] = struct{}{}
	}

	var conflictAllergens []string
	for _, ua := range profile.Allergens {
		if _, has := dishAllergens[ua]; has {
			label := allergenRU[ua]
			if label == "" {
				label = ua
			}
			conflictAllergens = append(conflictAllergens, label)
		}
	}

	var missingDietary []string
	for _, ud := range profile.Dietary {
		if _, has := dishDietary[ud]; !has {
			label := dietaryRU[ud]
			if label == "" {
				label = ud
			}
			missingDietary = append(missingDietary, label)
		}
	}

	parts := []string{}
	if len(conflictAllergens) > 0 {
		parts = append(parts, "содержит "+strings.Join(conflictAllergens, ", "))
	}
	if len(missingDietary) > 0 {
		parts = append(parts, "не "+strings.Join(missingDietary, ", "))
	}
	return strings.Join(parts, "; ")
}

// retrievedDishRaw минимальный набор полей блюда, нужный для computeRestrictionReason.
// Отдельный тип, чтобы не тащить полный usecasemodels.Dish в чисто RAG-функцию.
type retrievedDishRaw struct {
	allergens []string
	dietary   []string
}

// dishToText собирает текст блюда для рерэнкера в формате, идентичном embed-тексту
// в Qdrant (см. indexer.BuildEmbedText). Согласованность критична: если embed
// подтянул блюдо по pairing-фразе («Подходит к: белому вину»), а rerank
// переранжирует его по тексту без этой фразы — relevance score распределяется
// по описанию вкуса и состава, pairing-сигнал теряется и нужное блюдо роняется
// ниже шумного матча по lexical overlap'у (например, «Ризотто с белыми грибами»,
// у которого в composition буквально «белое сухое вино», бьёт правильное блюдо
// под запрос «к белому вину»).
func dishToText(d *usecasemodels.Dish, categoryName string) string {
	pairings := make([]indexer.PairingTagView, 0, len(d.PairingTags))
	for _, pt := range d.PairingTags {
		pairings = append(pairings, indexer.PairingTagView{
			Slug:        pt.Slug,
			Axis:        pt.Axis,
			EmbedPhrase: pt.EmbedPhrase,
		})
	}
	return indexer.BuildEmbedText(indexer.DishView{
		Name:         d.Name,
		Description:  d.Description,
		Composition:  d.Composition,
		Cuisine:      string(d.Cuisine),
		CategoryName: categoryName,
		PairingTags:  pairings,
	})
}

// cuisineLabel возвращает русское имя кухни или сам код если маппинга нет
func cuisineLabel(code string) string {
	if v, ok := cuisineRU[code]; ok {
		return v
	}
	return code
}

// boldSpanRe ловит контент markdown-обёрток **...**. Используется в
// recoverIDsFromText как «фильтр интента»: tail-reminder требует от LLM
// оборачивать названия блюд в bold, поэтому матчинг блюд ведётся только
// внутри этих спанов, а не по всему тексту ответа. Это защищает от
// false-positive на словах из описаний («эспрессо со льдом», «апельсиновым
// фрешем»), которые ложно матчили имена companions при поиске по всему тексту.
var boldSpanRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)

// recoverIDsFromText fallback-парсер: восстанавливает recommended_dish_ids
// по упоминаниям блюд в тексте, когда LLM не вернула финальный JSON-блок
// (или вернула пустой массив, но при этом назвала блюда в тексте).
//
// Принцип: ищем только внутри **bold**-спанов. Tail-reminder в промпте
// требует от LLM выделять названия блюд жирным — это её явный сигнал
// «это название, а не описание». Описательные слова («эспрессо со льдом»,
// «острый соус») в жирное не выделяются, поэтому ложно блюда они не сматчат.
//
// Внутри каждого spans пробуем двухуровневый матч:
//  1. Полное case-insensitive substring-совпадение имени с word-boundary
//     защитой — точно для номинатива («**Цезарь**»).
//  2. Стем-fallback: GigaChat склоняет имена («**Хлебную корзину**» вместо
//     «Хлебная корзина»). Срезаем падежные окончания и требуем, чтобы все
//     значимые стемы встретились в spans.
//
// Если жирных спанов в тексте нет — возвращаем nil. Лучше пусто, чем
// аггрессивно искать по всему тексту и попадать на false-positive.
func recoverIDsFromText(text string, main, companions []retrievedDish) []int {
	if text == "" {
		return nil
	}
	boldMatches := boldSpanRe.FindAllStringSubmatch(text, -1)
	if len(boldMatches) == 0 {
		return nil
	}
	spans := make([]string, len(boldMatches))
	for i, m := range boldMatches {
		spans[i] = strings.ToLower(m[1])
	}

	type candidate struct {
		id  int
		idx int // позиция в slice spans — отражает порядок появления в тексте
	}
	var found []candidate
	seen := make(map[int]struct{})

	check := func(d retrievedDish) {
		if d.name == "" {
			return
		}
		if _, dup := seen[d.id]; dup {
			return
		}
		needle := strings.ToLower(d.name)
		stems := significantStems(d.name)

		for i, span := range spans {
			// Уровень 1: полное вхождение имени блюда в bold-span.
			if idx := strings.Index(span, needle); idx >= 0 && boundaryOK(span, idx, len(needle)) {
				seen[d.id] = struct{}{}
				found = append(found, candidate{id: d.id, idx: i})
				return
			}
			// Уровень 2: все значимые стемы имени встречаются в span.
			if len(stems) == 0 {
				continue
			}
			allMatch := true
			for _, s := range stems {
				if !strings.Contains(span, s) {
					allMatch = false
					break
				}
			}
			if allMatch {
				seen[d.id] = struct{}{}
				found = append(found, candidate{id: d.id, idx: i})
				return
			}
		}
	}
	for _, d := range main {
		check(d)
	}
	for _, d := range companions {
		check(d)
	}

	// Порядок появления в тексте — чтобы id шли в том порядке, как LLM их назвал.
	sort.Slice(found, func(i, j int) bool { return found[i].idx < found[j].idx })

	out := make([]int, len(found))
	for i, c := range found {
		out[i] = c.id
	}
	return out
}

// stemMinLength минимальная длина стема (в рунах), чтобы он попал в матч.
// Стемы короче (например, «бо» от «Борщ») дают много false-positive: «бо» внутри
// «большой», «более», «бок». Лучше пропустить — у таких блюд остаётся только
// шанс на полное substring-совпадение (Уровень 1 в recoverIDsFromText).
const stemMinLength = 4

// significantStems извлекает «стемы» (приближённые корни) значимых слов имени блюда.
// Используется в recoverIDsFromText для матчинга склонённых форм: «Хлебную корзину»
// → стемы ["хлебн", "корзи"] совпадают с именем в БД «Хлебная корзина».
//
// Эвристика:
//   - слова ≤ 3 рун пропускаем (предлоги/союзы «в», «на», «и»);
//   - срезаем 2 последних символа у слов 4-8 рун и 3 у слов > 8 рун (покрывает
//     большинство русских падежных и числовых окончаний);
//   - стемы короче stemMinLength отбрасываем — слишком короткий «корень» даёт
//     ложные срабатывания на словах общей лексики.
//
// Возвращает lowercase-стемы. Пунктуация и не-буквы вокруг слов отбрасываются.
func significantStems(name string) []string {
	out := []string{}
	for _, w := range strings.Fields(strings.ToLower(name)) {
		trimmed := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		runes := []rune(trimmed)
		if len(runes) <= 3 {
			continue
		}
		cut := 2
		if len(runes) > 8 {
			cut = 3
		}
		stem := string(runes[:len(runes)-cut])
		if utf8.RuneCountInString(stem) < stemMinLength {
			continue
		}
		out = append(out, stem)
	}
	return out
}

// boundaryOK true если символы вокруг [start, start+length) не являются «продолжением слова»
// (буквы/цифры). Простая эвристика поверх unicode.IsLetter/IsDigit для UTF-8 имён.
func boundaryOK(s string, start, length int) bool {
	end := start + length
	if start > 0 {
		prevRune, _ := utf8.DecodeLastRuneInString(s[:start])
		if unicode.IsLetter(prevRune) || unicode.IsDigit(prevRune) {
			return false
		}
	}
	if end < len(s) {
		nextRune, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsLetter(nextRune) || unicode.IsDigit(nextRune) {
			return false
		}
	}
	return true
}

// parseLLMTail извлекает recommended_dish_ids из JSON-блока в ответе и удаляет блок из текста.
// Порядок попыток (от наиболее предпочтительного к запасному):
//  1. fenced с любыми backticks (тройной/двойной/одинарный fence) — берём последнее
//     вхождение, если модель почему-то вывела несколько блоков;
//  2. голый {...} в самом конце ответа — «канонический» формат при потерянных backticks;
//  3. голый {...} где угодно (последнее вхождение) — последний шанс.
func parseLLMTail(raw string) (string, []int) {
	if locs := jsonFencedRe.FindAllStringSubmatchIndex(raw, -1); len(locs) > 0 {
		last := locs[len(locs)-1]
		return extractIDs(raw, last[0], last[1], raw[last[2]:last[3]])
	}
	if loc := jsonTailRe.FindStringIndex(raw); loc != nil {
		return extractIDs(raw, loc[0], loc[1], raw[loc[0]:loc[1]])
	}
	if locs := jsonAnywhereRe.FindAllStringIndex(raw, -1); len(locs) > 0 {
		last := locs[len(locs)-1]
		return extractIDs(raw, last[0], last[1], raw[last[0]:last[1]])
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
