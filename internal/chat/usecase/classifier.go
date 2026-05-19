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
	// targetCategorySlug категория-цель cross-sell-запроса («гарнир к стейку» → side).
	// Один из validTargetCategorySlugs или пусто. ПРИ НЕПУСТОМ ЗНАЧЕНИИ runRAG
	// делает HARD-фильтр по category_id (без hybrid RRF — поиск ведётся внутри
	// этой категории по семантике). Это самый сильный сигнал — гость явно
	// указал, какую категорию ему нужно.
	targetCategorySlug string
	// priceIntent ценовой фокус. Один из validPriceIntents или пусто.
	// «cheap» — фильтр price_minor <= cheapPriceMaxMinor;
	// «premium» — фильтр price_minor >= premiumPriceMinMinor.
	priceIntent string
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
	res.targetCategorySlug = parsed.targetCategorySlug
	res.priceIntent = parsed.priceIntent

	log.Info("classified",
		"intent", string(res.intent),
		"pairing_drink", res.pairingDrink,
		"occasion", res.occasion,
		"target_category", res.targetCategorySlug,
		"price_intent", res.priceIntent,
		"latency_ms", res.latency.Milliseconds(),
		"raw", res.rawResponse)
	return res
}

// jsonObjectRe ловит первый JSON-объект в ответе классификатора.
// Толерантен к мусору вокруг (приветствие/обёрточные backticks/markdown).
var jsonObjectRe = regexp.MustCompile(`(?s)\{[^{}]*\}`)

// parsedClassification — структурированный выход parseClassifierResponse.
// Все поля кроме intent опциональны; невалидные значения вычищаются на этапе
// парсинга (defensive — невалидный slug = поле "", retrieval не активирует
// соответствующий механизм).
type parsedClassification struct {
	intent             intent
	pairingDrink       string
	occasion           string
	targetCategorySlug string
	priceIntent        string
}

// parseClassifierResponse разбирает ответ классификатора в parsedClassification + ok.
//
// Поддерживает два формата:
//
//  1. JSON:
//     {
//       "intent": "recommend",
//       "pairing_drink": "white_wine",       // опц., из validPairingDrinks
//       "occasion": "date",                  // опц., из validOccasions
//       "target_category": "side",           // опц., из validTargetCategorySlugs
//       "price_intent": "cheap"              // опц., из validPriceIntents
//     }
//     Поля могут быть null/отсутствовать. Невалидное значение → "" (fall-open),
//     intent сохраняется. Если intent невалиден — fall-back на word-format.
//
//  2. Голое слово: «recommend» / «clarify» / «chitchat» / «off_topic».
//     Для обратной совместимости с админ-промптом, который не обновляли.
//
// Сначала пробуем JSON (если есть {...} в ответе), потом fallback на слово.
func parseClassifierResponse(raw string) (parsedClassification, bool) {
	if raw == "" {
		return parsedClassification{}, false
	}
	// 1. JSON-формат
	if loc := jsonObjectRe.FindStringIndex(raw); loc != nil {
		var payload struct {
			Intent         string `json:"intent"`
			PairingDrink   string `json:"pairing_drink"`
			Occasion       string `json:"occasion"`
			TargetCategory string `json:"target_category"`
			PriceIntent    string `json:"price_intent"`
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
				if cat := strings.ToLower(strings.TrimSpace(payload.TargetCategory)); cat != "" {
					if _, valid := validTargetCategorySlugs[cat]; valid {
						out.targetCategorySlug = cat
					}
				}
				out.priceIntent = normalizePriceIntent(payload.PriceIntent)
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
