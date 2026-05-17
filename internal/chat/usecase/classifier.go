package usecase

import (
	"context"
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
	// rawResponse сырой ответ модели (для отладки в meta)
	rawResponse string
	// latency сколько занял classifier-вызов
	latency time.Duration
	// failed true, если classifier упал и мы фоллбэкнулись на fallbackIntent
	failed bool
	// failureReason описание причины фоллбэка (для meta/логов)
	failureReason string
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

	parsed, ok := parseIntent(res.rawResponse)
	if !ok {
		res.failed = true
		res.failureReason = "parse_failed: unknown response"
		log.Warn("classifier response did not match any intent, using fallback",
			"raw", res.rawResponse,
			"latency_ms", res.latency.Milliseconds(),
			"fallback", string(fallbackIntent))
		return res
	}
	res.intent = parsed

	log.Info("classified",
		"intent", string(res.intent),
		"latency_ms", res.latency.Milliseconds(),
		"raw", res.rawResponse)
	return res
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
