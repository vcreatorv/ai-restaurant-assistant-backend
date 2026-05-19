package usecase

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	"github.com/google/uuid"
)

// intent — намерение пользователя в одном сообщении.
// Определяется отдельным LLM-вызовом до основного pipeline.
type intent string

const (
	// intentRecommend — пользователь хочет получить новые рекомендации блюд.
	intentRecommend intent = "recommend"
	// intentClarify — уточнение/расспрос про уже обсуждаемое.
	intentClarify intent = "clarify"
	// intentChitchat — приветствие/благодарность/прощание без вопроса по меню.
	intentChitchat intent = "chitchat"
	// intentOffTopic — запрос не про ресторан и не про еду.
	intentOffTopic intent = "off_topic"
)

// validIntents — для парсинга ответа классификатора.
var validIntents = map[string]intent{
	string(intentRecommend): intentRecommend,
	string(intentClarify):   intentClarify,
	string(intentChitchat):  intentChitchat,
	string(intentOffTopic):  intentOffTopic,
}

// classifyResult выход classify() с телеметрией для записи в meta.
type classifyResult struct {
	// intent итоговая категория (всегда валидное значение; при сбое — fallbackIntent)
	intent intent
	// pairingDrink извлечённое из запроса намерение пейринга с напитком.
	// Один из validPairingDrinks или пусто. При непустом значении runRAG
	// запускает второй поток с filter pairing_tags=pair_<value> и сливает
	// результаты с основным семантическим поиском через RRF.
	pairingDrink string
	// occasion извлечённый повод/тип события. Один из validOccasions или пусто.
	// При непустом значении runRAG добавляет в Stream B фильтр по occasion_<value>.
	// Совместим с pairingDrink (можно запросить «на свидание под белое вино»).
	occasion string
	// targetCategorySlugs — массив категорий-целей. Заполняется, если гость
	// явно назвал одну или несколько категорий («гарнир к стейку» → ["side"];
	// «пицца с бургером» → ["pasta", "burger"]; «мясное из закусок» → ["starter"]).
	// При len(targetCategoryIDs)==1 в runRAG срабатывает cross-sell mode (HARD-фильтр
	// по одной категории, без rerank/diversify). При len>1 — multi-category mode:
	// раздельные Qdrant.Search'и по каждой категории, top-N из каждой объединяется.
	// До v2 classifier-промпта в БД это поле всегда пустое — модель отвечала
	// одним словом intent и не возвращала структурированный JSON.
	targetCategorySlugs []string
	// priceIntent ценовой фокус. Один из validPriceIntents или пусто.
	// «cheap» — фильтр price_minor <= cheapPriceMaxMinor;
	// «premium» — фильтр price_minor >= premiumPriceMinMinor.
	priceIntent string
	// mealStructure — паттерн трапезы, который гость подразумевает в запросе.
	// «full_dinner» / «full_lunch» — runRAG форсирует каркасный сбор по main-категориям
	// (закуска/салат → горячее → десерт), отключая порог MainDiversifyMinScore — иначе
	// на «сытный ужин» embed подтягивает 3 салата (лексический оверлап «сытный»),
	// и diversify не добивает суп/горячее из-за низкого cosine score.
	// «fastfood_set» — hard-filter по category_id ∈ (Бургеры, Гарниры, Закуски, Напитки) —
	// «фастфуд» в embed не матчится с «бургер» в описаниях меню, поэтому семантика не помогает.
	// «breakfast» / «snack» — пока без специальной логики retrieval, но сохраняем для логов.
	// Один из validMealStructures или пусто.
	mealStructure string
	// rawResponse сырой ответ модели (для отладки в meta)
	rawResponse string
	// latency сколько занял classifier-вызов
	latency time.Duration
	// failed true, если classifier упал и мы фоллбэкнулись на fallbackIntent
	failed bool
	// failureReason описание причины фоллбэка (для meta/логов)
	failureReason string
}

// validPairingDrinks — допустимые значения pairing_drink из classifier.
// Источник истины — slug'и в pairing_tags WHERE axis='drink' (миграция 000012),
// с префиксом pair_, который классификатор НЕ возвращает (короткие имена
// читаемее в промпте и в JSON).
var validPairingDrinks = map[string]string{
	"white_wine": "pair_white_wine",
	"red_wine":   "pair_red_wine",
	"sparkling":  "pair_sparkling",
	"beer_light": "pair_beer_light",
	"beer_dark":  "pair_beer_dark",
	"cider":      "pair_cider",
	"cocktails":  "pair_cocktails",
	"coffee":     "pair_coffee",
	"tea":        "pair_tea",
	"lemonade":   "pair_lemonade",
}

// pairingDrinkToSlug маппит короткое значение classifier'а в pairing_tags.slug.
// Возвращает "" для неизвестных значений (defensive — невалидный slug = фильтр
// не применяется, retrieval идёт как обычный single-stream).
func pairingDrinkToSlug(v string) string {
	if v == "" {
		return ""
	}
	return validPairingDrinks[strings.ToLower(strings.TrimSpace(v))]
}

// validOccasions — допустимые значения occasion из classifier.
// Источник истины — slug'и в pairing_tags WHERE axis='occasion' (без префикса
// occasion_, который классификатор не пишет — короткие имена читаемее).
var validOccasions = map[string]string{
	"date":           "occasion_date",
	"celebration":    "occasion_celebration",
	"business_lunch": "occasion_business_lunch",
	"kids":           "occasion_kids",
	"breakfast":      "occasion_breakfast",
}

// occasionToSlug — alias-резолвер для occasion.
func occasionToSlug(v string) string {
	if v == "" {
		return ""
	}
	return validOccasions[strings.ToLower(strings.TrimSpace(v))]
}

// validTargetCategorySlugs — допустимые значения target_category из classifier.
// Cross-sell-запросы вида «гарнир к стейку», «соус к мясу», «десерт после
// рибая» — классификатор должен извлечь target_category из таких запросов,
// и runRAG применит HARD-фильтр по соответствующей категории в Qdrant.
// Маппинг slug → имя категории в БД делает chat usecase через categoryByName.
var validTargetCategorySlugs = map[string]string{
	"starter":  "Закуски",
	"salad":    "Салаты",
	"soup":     "Супы",
	"side":     "Гарниры",
	"sauce":    "Соусы",
	"dessert":  "Десерты",
	"drink":    "Напитки",
	"alcohol":  "Алкоголь",
	"seafood":  "Морепродукты",
	"steak":    "Стейки",
	"burger":   "Бургеры",
	"pasta":    "Пицца и паста",
	"grill":    "На гриле",
	"meat":     "Мясное",
}

// targetCategoryToName — резолвер slug → имя категории. Имя совпадает с
// categories.name в БД (case-sensitive). Возвращает "" для неизвестных.
func targetCategoryToName(v string) string {
	if v == "" {
		return ""
	}
	return validTargetCategorySlugs[strings.ToLower(strings.TrimSpace(v))]
}

// singleTargetCategoryName возвращает русское имя категории, ТОЛЬКО если в
// массиве ровно один slug. Используется для tail-reminder в prompt'е («Гость
// спросил конкретно про категорию X — дай 2-3 варианта на выбор»): на multi-
// target запросе «пицца с бургером» формулировка «про категорию Пицца и паста, Бургеры»
// звучит криво и не несёт смысла, проще оставить общее поведение system-промпта.
func singleTargetCategoryName(slugs []string) string {
	if len(slugs) != 1 {
		return ""
	}
	return targetCategoryToName(slugs[0])
}

// validPriceIntents — допустимые значения price_intent.
//
// cheap — гость явно хочет недорогое («что подешевле», «бюджетное»).
//
// premium — гость явно хочет премиум («самое дорогое», «топовое», «лучшее»).
// На «средний чек» / «как обычно» оба intent'а пустые — не вмешиваемся.
var validPriceIntents = map[string]struct{}{
	"cheap":   {},
	"premium": {},
}

// validMealStructures — допустимые значения meal_structure.
//
// full_dinner / full_lunch — гость хочет полноценный приём пищи. runRAG форсирует
// каркасный сбор: top-1 из каждой непокрытой main-категории (см. runMainDiversification),
// порог MainDiversifyMinScore временно отключён.
//
// fastfood_set — гость явно сказал «фастфуд» / «как в Макдональдсе». В embed это
// плохо матчится с «бургер»/«крылышки» из описаний меню (описания маркетинговые,
// слово «фастфуд» в них не появляется). Поэтому переключаемся в multi-category
// mode с set'ом (Бургеры, Гарниры, Закуски, Напитки).
//
// breakfast / snack — пока без специальной retrieval-логики, сохраняем для логов
// и аналитики (можно подключить позже).
var validMealStructures = map[string]struct{}{
	"full_dinner":  {},
	"full_lunch":   {},
	"fastfood_set": {},
	"breakfast":    {},
	"snack":        {},
}

// fastfoodCategorySlugs — какие target-категории разворачивает meal_structure="fastfood_set".
// Сюда не входят Стейки/Гриль/Мясное/Пицца и паста — они «полноценное горячее», не fastfood.
// Алкоголь / Десерты / Салаты / Супы / Соусы тоже не fastfood.
var fastfoodCategorySlugs = []string{"burger", "side", "starter", "drink"}

// normalizeMealStructure — фильтр против невалидных значений.
func normalizeMealStructure(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if _, ok := validMealStructures[v]; ok {
		return v
	}
	return ""
}

// normalizePriceIntent — фильтр против невалидных значений.
func normalizePriceIntent(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if _, ok := validPriceIntents[v]; ok {
		return v
	}
	return ""
}

// fallbackIntent — куда фоллбэчимся при сбое классификатора.
// recommend безопаснее всего: даже если на самом деле chitchat — пользователь
// получит лишний RAG-вызов, но не получит «не могу помочь» вместо рекомендации.
const fallbackIntent = intentRecommend

// classify прогоняет короткий LLM-вызов классификатора и возвращает intent.
// При любой ошибке возвращает fallbackIntent и пишет в result.failed=true.
// Не возвращает error — отказ классификатора не должен валить отправку сообщения.
func (uc *chatUsecase) classify(ctx context.Context, userText string, userID uuid.UUID) classifyResult {
	log := logger.ForCtx(ctx).With("stage", "classify")
	start := time.Now()
	res := classifyResult{intent: fallbackIntent}

	template, err := uc.prompts.GetActive(ctx, prompts.NameClassification, userID)
	if err != nil {
		res.failed = true
		res.failureReason = "prompt_load_failed: " + err.Error()
		log.Warn("classifier prompt unavailable, using fallback intent",
			"err", err, "fallback", string(fallbackIntent))
		return res
	}

	prompt := strings.ReplaceAll(template, "{{user_message}}", userText)
	var raw strings.Builder
	_, err = uc.classifier.ChatStream(ctx, llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
	}, func(delta string) error {
		raw.WriteString(delta)
		return nil
	})
	res.latency = time.Since(start)
	res.rawResponse = strings.TrimSpace(raw.String())

	if err != nil {
		res.failed = true
		res.failureReason = "llm_failed: " + err.Error()
		log.Warn("classifier llm failed, using fallback intent",
			"err", err,
			"latency_ms", res.latency.Milliseconds(),
			"fallback", string(fallbackIntent))
		return res
	}

	parsed, ok := parseClassifierResponse(res.rawResponse)
	if !ok {
		res.failed = true
		res.failureReason = "parse_failed: unknown response"
		log.Warn("classifier response did not match any intent, using fallback",
			"raw", res.rawResponse,
			"latency_ms", res.latency.Milliseconds(),
			"fallback", string(fallbackIntent))
		return res
	}
	res.intent = parsed.intent
	res.pairingDrink = parsed.pairingDrink
	res.occasion = parsed.occasion
	res.targetCategorySlugs = parsed.targetCategorySlugs
	res.priceIntent = parsed.priceIntent
	res.mealStructure = parsed.mealStructure

	log.Info("classified",
		"intent", string(res.intent),
		"pairing_drink", res.pairingDrink,
		"occasion", res.occasion,
		"target_categories", res.targetCategorySlugs,
		"price_intent", res.priceIntent,
		"meal_structure", res.mealStructure,
		"latency_ms", res.latency.Milliseconds(),
		"raw", res.rawResponse)
	return res
}

// jsonObjectRe ловит первый JSON-объект в ответе классификатора.
// Толерантен к мусору вокруг (приветствие/обёрточные backticks/markdown).
// `[^{}]*` НЕ исключает квадратные скобки, поэтому массивы строк
// (target_categories: ["x","y"]) и null-значения попадают в матч корректно.
var jsonObjectRe = regexp.MustCompile(`(?s)\{[^{}]*\}`)

// parsedClassification — структурированный выход parseClassifierResponse.
// Все поля кроме intent опциональны; невалидные значения вычищаются на этапе
// парсинга (defensive — невалидный slug = поле "" / пустой массив, retrieval
// не активирует соответствующий механизм).
type parsedClassification struct {
	intent              intent
	pairingDrink        string
	occasion            string
	targetCategorySlugs []string
	priceIntent         string
	mealStructure       string
}

// parseClassifierResponse разбирает ответ классификатора в parsedClassification + ok.
//
// Поддерживает два формата:
//
//  1. JSON (v2 промпта, миграция 000013):
//     {
//       "intent": "recommend",
//       "pairing_drink":     "white_wine",       // опц., из validPairingDrinks
//       "occasion":          "date",             // опц., из validOccasions
//       "target_categories": ["pasta","burger"], // опц., массив из validTargetCategorySlugs
//       "target_category":   "side",             // опц., legacy v1.5 — одиночное значение
//                                                //       (для совместимости со старыми
//                                                //       администраторскими черновиками)
//       "price_intent":      "cheap",            // опц., из validPriceIntents
//       "meal_structure":    "full_dinner"       // опц., из validMealStructures
//     }
//     Поля могут быть null/отсутствовать. Невалидные значения отбрасываются,
//     intent сохраняется. Если intent невалиден — fall-back на word-format.
//     При наличии и target_categories, и target_category — берётся target_categories
//     (он более выразителен и появился позже).
//
//  2. Голое слово: «recommend» / «clarify» / «chitchat» / «off_topic».
//     Для обратной совместимости с v1 промпта (миграция 000008).
//
// Сначала пробуем JSON (если есть {...} в ответе), потом fallback на слово.
func parseClassifierResponse(raw string) (parsedClassification, bool) {
	if raw == "" {
		return parsedClassification{}, false
	}
	// 1. JSON-формат
	if loc := jsonObjectRe.FindStringIndex(raw); loc != nil {
		var payload struct {
			Intent           string   `json:"intent"`
			PairingDrink     string   `json:"pairing_drink"`
			Occasion         string   `json:"occasion"`
			TargetCategories []string `json:"target_categories"`
			TargetCategory   string   `json:"target_category"` // legacy single-value
			PriceIntent      string   `json:"price_intent"`
			MealStructure    string   `json:"meal_structure"`
		}
		if err := json.Unmarshal([]byte(raw[loc[0]:loc[1]]), &payload); err == nil {
			if v, ok := validIntents[strings.ToLower(payload.Intent)]; ok {
				out := parsedClassification{intent: v}
				if drink := strings.ToLower(strings.TrimSpace(payload.PairingDrink)); drink != "" {
					if _, valid := validPairingDrinks[drink]; valid {
						out.pairingDrink = drink
					}
				}
				if occ := strings.ToLower(strings.TrimSpace(payload.Occasion)); occ != "" {
					if _, valid := validOccasions[occ]; valid {
						out.occasion = occ
					}
				}
				out.targetCategorySlugs = normalizeTargetCategorySlugs(payload.TargetCategories, payload.TargetCategory)
				out.priceIntent = normalizePriceIntent(payload.PriceIntent)
				out.mealStructure = normalizeMealStructure(payload.MealStructure)
				return out, true
			}
		}
		// JSON есть, но intent невалиден — продолжаем как word-format
	}
	// 2. Word-only fallback
	v, ok := parseIntent(raw)
	if !ok {
		return parsedClassification{}, false
	}
	return parsedClassification{intent: v}, true
}

// normalizeTargetCategorySlugs нормализует target_categories (массив) + legacy
// target_category (один slug) в один отсортированный по приходу uniq-массив
// валидных slug'ов. Невалидные значения тихо отбрасываются (fall-open).
//
// Если modern и legacy поля оба заполнены — modern (array) выигрывает целиком,
// legacy игнорируется. Это намеренно: если новый prompt вернул массив, его и
// слушаем; путаница «и то и то» — это, скорее всего, баг prompt'а.
func normalizeTargetCategorySlugs(arr []string, legacy string) []string {
	src := arr
	if len(src) == 0 && strings.TrimSpace(legacy) != "" {
		src = []string{legacy}
	}
	if len(src) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(src))
	out := make([]string, 0, len(src))
	for _, raw := range src {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if _, valid := validTargetCategorySlugs[v]; !valid {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseIntent ищет в ответе классификатора любое из четырёх валидных слов
// (модель может ответить «recommend», «recommend.», «**recommend**» — всё ок).
// Берём первое совпадение в нормализованном виде (lower, без пунктуации).
func parseIntent(raw string) (intent, bool) {
	if raw == "" {
		return "", false
	}
	low := strings.ToLower(raw)
	// Сначала пробуем «весь ответ — один токен», без разбивки.
	trimmed := strings.TrimFunc(low, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '_'
	})
	if v, ok := validIntents[trimmed]; ok {
		return v, true
	}
	// Иначе разбиваем по пунктуации и ищем первый валидный токен.
	for _, word := range strings.FieldsFunc(low, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '_'
	}) {
		if v, ok := validIntents[word]; ok {
			return v, true
		}
	}
	return "", false
}
