package usecase

import (
	"fmt"
	"strings"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
)

// systemPrompt инструкция модели; правила приоритетов (current > hard constraints > soft prefs)
const systemPrompt = `Ты — официант ресторана THE BULL. Помогаешь гостям выбрать блюда из меню — естественно, без фамильярности, как это делает живой официант.

Правила:
1. Рекомендуй ТОЛЬКО блюда из списка ниже (раздел «Меню» и раздел «Сопровождение»). Никогда не выдумывай.
2. Если подходящего нет — честно скажи и предложи близкое.
3. Аллергены и диета гостя — жёсткое ограничение. Контекст уже отфильтрован — НЕ упоминай это в ответе («не содержит молочных продуктов», «без глютена для вас» и подобное запрещено).
4. В тексте называй блюдо по имени, БЕЗ id и БЕЗ цены. Цену озвучивай только если гость спросил про неё или про бюджет.
5. На «спасибо», «пока», «понятно» — короткий ответ без рекомендаций.
6. На вопросы не про меню — одной фразой переключи на меню.
7. Без эмодзи и рекламных штампов. Запрещены формулировки: «изумительный», «восхитительный», «непременно», «разогнать аппетит», «разогреть аппетит», «идеальный способ», «идеальное начало», «идеально подойдёт», «настоящее наслаждение», «откроет/откроют ваш(е) ужин/вечер», «не оставит равнодушным» и любые подобные клише в духе ресторанной рекламы.

Стиль рекомендации:
8. Подача должна быть «вкусной». Используй прилагательные текстуры и вкуса (хрустящий, нежный, сочный, кремовый, пряный, ароматный, насыщенный, сладко-острый, золотистый), эпитеты к ингредиентам (мраморная говядина, спелые томаты, солоноватая брынза, ароматная паста том-ям) и короткие глагольные ноты (согревает, оттеняет мясо, даёт контраст, закрывает ужин). Не сводись к голому перечислению состава.
9. Если в названии блюда есть незнакомое слово (фокачча, бабагануш, рататуй, том-ям, пиканья, чизкейк баскский и т.п.) — нативно поясни через тире: «фокачча — итальянская пшеничная лепёшка с …», «бабагануш — восточная закуска из печёного баклажана с …». Не выноси пояснение в скобки и не делай отдельным предложением «это блюдо такое-то».
10. На широкий запрос (ужин, что-нибудь лёгкое, что взять) предлагай 3–4 блюда в порядке подачи (старт → горячее → гарнир/дополнение → десерт). На узкий запрос (один конкретный тип) — 1–3 варианта.
11. Связки между блюдами варьируй: «На старт», «Для начала», «На горячее», «К ней», «В сопровождение», «Отдельно стоит попробовать», «В финал», «На завершение». Не повторяй одну и ту же конструкцию во всём ответе и не используй формулу «На X — Y» подряд.
12. Отвечай по-русски, естественным живым языком. Длина — по запросу: на короткий вопрос про одно блюдо 1–2 предложения, на сборку ужина 3–5 предложений.

ВАЖНО про формат вывода:
- НЕ размышляй вслух. Не пиши «давайте подумаем», «сначала рассмотрим варианты», «учитывая...» и подобное. Сразу финальная рекомендация.
- НЕ повторяй один и тот же тезис разными словами. Каждое предложение — новая информация.
- Финальный JSON-блок (см. правило 22) ОБЯЗАТЕЛЕН. Резерв ~80 токенов специально под него — закончи текст до того, как он понадобится. Если чувствуешь, что лимит близко — обрывай красиво и сразу пиши JSON.

Сочетания (используй блок «Сопровождение» из контекста, если уместно к запросу):
13. К супу — предложи хлеб/фокаччу.
14. К горячему мясу/стейку/гриль — предложи соус, если в составе блюда соуса нет.
15. К горячей рыбе — предложи лимонад или сок (свежевыжатый).
16. К пицце — предложи лимонад или сок.
17. К острому азиатскому — предложи лимонад или сок.
18. К десерту — предложи чай или кофе.
19. К завтраку (омлет, гранола) — предложи кофе или чай.
20. Алкоголь предлагай ТОЛЬКО если гость явно попросил («что выпить», «вино к стейку», «есть пиво»). В разделе «Сопровождение» алкоголь не появляется — не добавляй его сам.
21. Сопровождение добавляй короткой фразой («К ней — соус …», «Из напитков можно взять …»). Не навязывай: если для запроса гостя сопровождение неуместно (короткое уточнение, прощание, off-topic) — не добавляй.

22. В самом конце ответа отдельной строкой ОБЯЗАТЕЛЬНО добавь блок (ничего после него):
   ` + "```json\n{\"recommended_dish_ids\":[<id блюд, упомянутых в ответе>]}\n```" + `
   Если ничего не рекомендуешь — пустой массив. Перечисляй ровно те id, которые встречаются в твоём тексте, включая блюда из «Сопровождения», если ты их упомянул.`

// promptInput аргументы для сборки prompt'а LLM
type promptInput struct {
	// profile профиль текущего пользователя (allergens, dietary)
	profile *usecasemodels.User
	// retrieved главные блюда из RAG (top-N после рерэнкера)
	retrieved []retrievedDish
	// companions сопровождающие блюда (по 1 из каждой companion-категории)
	companions []retrievedDish
	// priorRecommended id блюд, рекомендованных в предыдущих сообщениях ассистента
	priorRecommended []int
	// anchor хронологически первый user-msg чата (или nil) — закрепляет рамку диалога
	anchor *repositorymodels.Message
	// history последние HistoryRecentPairs*2 сообщений диалога (без system, без текущего user)
	history []repositorymodels.Message
	// currentUserText текст текущего сообщения пользователя
	currentUserText string
}

// retrievedDish одно блюдо из выдачи RAG, с минимально нужными для prompt'а полями.
// Аллергены сюда не попадают: предфильтр Qdrant уже отрезал блюда с запрещёнными,
// и упоминать «не содержит X» в ответе LLM запрещено правилом 3.
type retrievedDish struct {
	// id идентификатор блюда
	id int
	// name название
	name string
	// description описание
	description string
	// composition состав
	composition string
	// categoryName русское название категории
	categoryName string
	// cuisineRU русское название кухни
	cuisineRU string
	// priceMinor цена в копейках
	priceMinor int
}

// buildPrompt собирает массив llm.Message: system + anchor + recent + user-msg с RAG-контекстом.
// На выходе формат, совместимый с OpenAI chat-completions (используется и OpenRouter, и NVIDIA NIM).
func buildPrompt(in promptInput) []llm.Message {
	out := make([]llm.Message, 0, len(in.history)+3)
	out = append(out, llm.Message{Role: llm.RoleSystem, Content: systemPrompt})

	if in.anchor != nil {
		role := mapHistoryRole(in.anchor.Role)
		if role != "" {
			out = append(out, llm.Message{Role: role, Content: in.anchor.Content})
		}
	}

	for i := range in.history {
		m := &in.history[i]
		role := mapHistoryRole(m.Role)
		if role == "" {
			continue
		}
		out = append(out, llm.Message{Role: role, Content: m.Content})
	}

	out = append(out, llm.Message{
		Role:    llm.RoleUser,
		Content: buildUserContent(in),
	})
	return out
}

// mapHistoryRole конвертирует role из БД в llm.Role; system-сообщения из истории игнорируем
func mapHistoryRole(role string) llm.Role {
	switch usecasemodels.MessageRole(role) {
	case usecasemodels.RoleUser:
		return llm.RoleUser
	case usecasemodels.RoleAssistant:
		return llm.RoleAssistant
	}
	return ""
}

// buildUserContent собирает финальный user-prompt: меню-контекст + текущий вопрос.
// Профиль гостя в prompt НЕ включается: hard-фильтры (аллергены, диета) уже применены
// pre-filter'ом в Qdrant; soft-предпочтения добавим позже как отдельный слой, если будет нужно.
func buildUserContent(in promptInput) string {
	var b strings.Builder

	if len(in.retrieved) > 0 {
		b.WriteString("\n=== Меню (наиболее подходящие блюда) ===\n")
		for i, d := range in.retrieved {
			fmt.Fprintf(&b, "\n%d. %s [id=%d] — %s, %s. Цена: %s.\n",
				i+1, d.name, d.id, d.categoryName, d.cuisineRU, formatPrice(d.priceMinor))
			if d.description != "" {
				fmt.Fprintf(&b, "   %s\n", d.description)
			}
			if d.composition != "" {
				fmt.Fprintf(&b, "   Состав: %s\n", d.composition)
			}
		}
		b.WriteString("\nid блюд использовать ТОЛЬКО в финальном JSON-блоке. В тексте называй блюда по имени.\n")
	} else {
		b.WriteString("\n=== Меню ===\nПо запросу подходящих блюд не найдено.\n")
	}

	if len(in.companions) > 0 {
		b.WriteString("\n=== Сопровождение (на выбор, если уместно к запросу) ===\n")
		for _, d := range in.companions {
			fmt.Fprintf(&b, "- %s [id=%d] (%s) — %s",
				d.name, d.id, d.categoryName, d.description)
			if d.composition != "" {
				fmt.Fprintf(&b, " Состав: %s.", d.composition)
			}
			b.WriteString("\n")
		}
		b.WriteString("Используй эти позиции как аккомпанемент к основным блюдам по правилам Сочетаний (см. system).\n")
	}

	if len(in.priorRecommended) > 0 {
		fmt.Fprintf(&b, "\n=== Ранее рекомендовано в этом диалоге ===\n%s\n",
			joinInts(in.priorRecommended))
	}

	b.WriteString("\n=== Вопрос гостя ===\n")
	b.WriteString(in.currentUserText)
	return b.String()
}

// formatPrice форматирует цену в копейках в человеческий вид (45000 → "450 ₽")
func formatPrice(minor int) string {
	if minor <= 0 {
		return ""
	}
	return fmt.Sprintf("%d ₽", minor/100)
}

// joinInts собирает []int в строку через запятую
func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ", ")
}
