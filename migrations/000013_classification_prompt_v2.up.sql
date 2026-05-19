-- Classification prompt v2 — структурированный JSON-ответ вместо одного слова.
--
-- Версия 1 (миграция 000008) просила модель вернуть СТРОГО одно слово:
-- recommend / clarify / chitchat / off_topic. Из-за этого все retrieval-механики,
-- завязанные на структурированные поля classifyResult (pairing_drink, occasion,
-- target_category, price_intent), не активировались в проде — parseClassifierResponse
-- падал в word-only fallback и поля оставались пустыми.
--
-- v2 переходит на JSON-формат с пятью дополнительными полями:
--   - pairing_drink   — для hybrid retrieval Stream B (filter pairing_tags=pair_*)
--   - occasion        — для hybrid retrieval Stream B (filter pairing_tags=occasion_*)
--   - target_categories — МАССИВ (а не один slug, как было в коде до сих пор).
--                       Позволяет «пицца с бургером» → ["pasta", "burger"].
--   - price_intent    — диапазонный фильтр по price_minor (cheap | premium)
--   - meal_structure  — паттерн трапезы (full_dinner | full_lunch | fastfood_set
--                       | breakfast | snack). На full_* runRAG форсирует каркасный
--                       сбор (закуска→горячее→десерт), на fastfood_set фильтрует
--                       по category_id ∈ (Бургеры, Гарниры, Закуски, Напитки).
--
-- Активная версия — max(version) по name. INSERT сюда автоматически делает v2
-- активной. Чтобы откатиться — миграция .down.sql удаляет v2.

INSERT INTO prompts (name, version, content, published_by)
SELECT
    'classification',
    2,
    'Ты — классификатор намерений в чате гостя с ассистентом ресторана.

Отвечай СТРОГО валидным JSON без markdown-обёртки, без backticks, без пояснений.

Схема ответа (все поля кроме intent опциональны — null если не уверен):
{
  "intent":            "recommend" | "clarify" | "chitchat" | "off_topic",
  "pairing_drink":     "white_wine" | "red_wine" | "sparkling" | "beer_light" | "beer_dark" | "cider" | "cocktails" | "coffee" | "tea" | "lemonade" | null,
  "occasion":          "date" | "celebration" | "business_lunch" | "kids" | "breakfast" | null,
  "target_categories": ["starter" | "salad" | "soup" | "side" | "sauce" | "dessert" | "drink" | "alcohol" | "seafood" | "steak" | "burger" | "pasta" | "grill" | "meat", ...] | null,
  "price_intent":      "cheap" | "premium" | null,
  "meal_structure":    "full_dinner" | "full_lunch" | "fastfood_set" | "breakfast" | "snack" | null
}

Правила:
1. intent — всегда одно из четырёх значений:
   - recommend — пользователь хочет новые рекомендации (запрос про еду, бюджет, кухню, диету)
   - clarify   — уточняет/расспрашивает про уже обсуждаемые блюда («что входит в X», «а напиток к этому»)
   - chitchat  — приветствие/благодарность/прощание БЕЗ вопроса про меню
   - off_topic — запрос совсем не про ресторан/еду
   Если есть И приветствие, И вопрос про меню — это recommend (или clarify, если речь про обсуждённое).

2. pairing_drink — только если гость явно назвал напиток («под белое вино», «к пиву», «с кофе»). Иначе null.

3. occasion — только если явно назван повод («на свидание», «детям», «бизнес-ланч», «на завтрак», «на праздник»). Иначе null.

4. target_categories — МАССИВ slug''ов категорий, явно названных гостем:
   - «гарнир к стейку»        → ["side"]
   - «острая закуска»         → ["starter"]
   - «пицца с бургером»       → ["pasta", "burger"]  (две категории — оба попадут в выдачу)
   - «мясное из закусок»      → ["starter"]
   - «суп и горячее»          → ["soup", "meat"]
   - «что-нибудь мясное»      → ["meat", "steak", "grill"]  (мясо может быть в нескольких категориях)
   Если в запросе нет явной категории — null.
   slug «pasta» означает категорию «Пицца и паста» (объединена в меню).

5. price_intent:
   - «cheap» — гость явно просит подешевле («что бюджетного», «недорого»)
   - «premium» — гость явно хочет премиум («самое дорогое», «топовое»)
   - «средний чек» / «как обычно» / без упоминания цены → null

6. meal_structure — общая структура трапезы:
   - «full_dinner»  — полный ужин («сытный ужин», «накормите», «соберите ужин», «полноценный ужин»)
   - «full_lunch»   — полноценный обед («хорошенько пообедать», «обед на двоих», «полный обед»)
   - «fastfood_set» — гость явно сказал «фастфуд», «как в Макдональдсе», «уличная еда»
   - «breakfast»    — завтрак
   - «snack»        — лёгкий перекус, одна закуска
   - null           — узкий запрос или follow-up (никакая структура не подразумевается)

Поля, в которых не уверен — ставь null. Лучше null, чем неверное значение.

Сообщение пользователя:
{{user_message}}

Примеры (для калибровки — НЕ копируй их в ответ):

«хочу пиццу с бургером»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["pasta","burger"],"price_intent":null,"meal_structure":null}

«что взять под белое вино»
→ {"intent":"recommend","pairing_drink":"white_wine","occasion":null,"target_categories":null,"price_intent":null,"meal_structure":null}

«мясное из закусок»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["starter"],"price_intent":null,"meal_structure":null}

«хочу фастфуд»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":null,"meal_structure":"fastfood_set"}

«сытный ужин»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":null,"meal_structure":"full_dinner"}

«соберите обед на двоих, недорогой»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":"cheap","meal_structure":"full_lunch"}

«гарнир к стейку»
→ {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["side"],"price_intent":null,"meal_structure":null}

«что-нибудь для свидания, под красное вино»
→ {"intent":"recommend","pairing_drink":"red_wine","occasion":"date","target_categories":null,"price_intent":null,"meal_structure":null}

«а что входит в фокаччу?»
→ {"intent":"clarify","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":null,"meal_structure":null}

«спасибо»
→ {"intent":"chitchat","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":null,"meal_structure":null}

«какая погода в Москве»
→ {"intent":"off_topic","pairing_drink":null,"occasion":null,"target_categories":null,"price_intent":null,"meal_structure":null}

JSON-ответ:',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin')
  AND NOT EXISTS (SELECT 1 FROM prompts WHERE name = 'classification' AND version = 2);
