-- Расширение classification prompt'а: + target_categories (массив) + meal_structure.
--
-- Контекст. В таблице prompts уже сидит работающий JSON-classifier (его опубликовали
-- через админ-UI поверх word-only v1 из миграции 000008). Текущий prompt возвращает
-- intent + pairing_drink + occasion + target_category (single) + price_intent.
-- Соответствующие retrieval-механики (Stream B hybrid, cross-sell hard-filter,
-- price-range) работают в проде.
--
-- Что добавляем:
--   - target_categories — МАССИВ slug''ов (а не один). Позволяет «пицца с бургером»
--     → ["pasta", "burger"], код в classifier.go::parseClassifierResponse уже умеет
--     читать оба `target_categories` (array) и legacy `target_category` (string).
--   - meal_structure — паттерн трапезы (full_dinner | full_lunch | fastfood_set
--     | breakfast | snack | null). runRAG включает forceFullCoverage в
--     runMainDiversification на full_*, и разворачивает fastfood_set в дефолтный
--     набор категорий (бургеры/гарниры/закуски/напитки).
--
-- Поля v1 (target_category) и примеры на них сохраняем для совместимости — код
-- читает их через payload.TargetCategory как legacy single-value fallback.
--
-- Активная версия = max(version). Чтобы не зависеть от того, какая версия
-- сейчас в проде у того или иного окружения, вычисляем next_version динамически
-- как (max(version)+1). После накатки эта запись становится активной автоматически.

INSERT INTO prompts (name, version, content, published_by)
SELECT
    'classification',
    COALESCE((SELECT MAX(version) FROM prompts WHERE name = 'classification'), 0) + 1,
    'Определи намерение гостя и (опционально) извлеки структурированные параметры запроса.
Верни строго JSON-объект в одну строку, без markdown и без дополнительного текста.

Схема:
{
  "intent": "recommend" | "clarify" | "chitchat" | "off_topic",
  "pairing_drink": "white_wine" | "red_wine" | "sparkling" | "beer_light" | "beer_dark" | "cider" | "cocktails" | "coffee" | "tea" | "lemonade" | null,
  "occasion": "date" | "celebration" | "business_lunch" | "kids" | "breakfast" | null,
  "target_categories": ["starter" | "salad" | "soup" | "side" | "sauce" | "dessert" | "drink" | "alcohol" | "seafood" | "steak" | "burger" | "pasta" | "grill" | "meat", ...] | null,
  "target_category": "starter" | "salad" | "soup" | "side" | "sauce" | "dessert" | "drink" | "alcohol" | "seafood" | "steak" | "burger" | "pasta" | "grill" | "meat" | null,
  "price_intent": "cheap" | "premium" | null,
  "meal_structure": "full_dinner" | "full_lunch" | "fastfood_set" | "breakfast" | "snack" | null
}

intent:
- recommend  — гость хочет совет/рекомендацию блюд
- clarify    — гость уточняет про блюдо или просит детали («что входит», «расскажи про X»)
- chitchat   — приветствие/благодарность/прощание
- off_topic  — запрос не про еду/ресторан

pairing_drink — если гость указал напиток, под который ищет блюда («под белое вино», «к пиву»)

occasion — если гость указал повод («на свидание», «детям», «на завтрак»)

target_categories — МАССИВ slug''ов, если гость явно назвал одну или несколько категорий:
- «гарнир к стейку»     → ["side"]
- «мясное из закусок»   → ["starter"]
- «пицца с бургером»    → ["pasta", "burger"]    (две категории — обе должны быть в выдаче)
- «суп и горячее»       → ["soup", "meat"]
- «что-нибудь мясное»   → ["meat", "steak", "grill"]   (мясо может быть в нескольких категориях)
Slug «pasta» означает категорию «Пицца и паста» (объединена в меню).
Если категория ОДНА — можно дополнительно продублировать в target_category (старое поле, для совместимости).
Если не уверен — null.

target_category — legacy single-value поле. Заполняй ТОЛЬКО когда target_categories содержит ровно один элемент,
и дублируй его сюда. На multi-target — null. Поле сохранено для совместимости со старым кодом.

price_intent:
- «cheap»   — гость явно просит подешевле («что бюджетного», «недорого»)
- «premium» — гость явно хочет премиум («самое дорогое», «топовое», «лучшее»)

meal_structure — общая структура трапезы:
- «full_dinner»  — полный ужин («сытный ужин», «накормите», «соберите ужин», «полноценный ужин»)
- «full_lunch»   — полноценный обед («хорошенько пообедать», «обед на двоих», «полный обед»)
- «fastfood_set» — фастфуд («хочу фастфуд», «как в Макдональдсе», «уличная еда»)
- «breakfast»    — завтрак
- «snack»        — лёгкий перекус, одна закуска
- null           — узкий запрос или follow-up без явной структуры

В остальных случаях каждое опциональное поле — null.

Примеры:
- «Подойдёт под белое вино» → {"intent":"recommend","pairing_drink":"white_wine","occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":null}
- «Нужен гарнир к стейку» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["side"],"target_category":"side","price_intent":null,"meal_structure":null}
- «Какой соус к мясу?» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["sauce"],"target_category":"sauce","price_intent":null,"meal_structure":null}
- «Что на десерт» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["dessert"],"target_category":"dessert","price_intent":null,"meal_structure":null}
- «Хочу чего-то выпить» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["drink"],"target_category":"drink","price_intent":null,"meal_structure":null}
- «На свидание под белое вино» → {"intent":"recommend","pairing_drink":"white_wine","occasion":"date","target_categories":null,"target_category":null,"price_intent":null,"meal_structure":null}
- «Что-то премиальное на праздник» → {"intent":"recommend","pairing_drink":null,"occasion":"celebration","target_categories":null,"target_category":null,"price_intent":"premium","meal_structure":null}
- «Бюджетный обед» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":"cheap","meal_structure":"full_lunch"}
- «Расскажи про карбонару» → {"intent":"clarify","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":null}
- «Привет» → {"intent":"chitchat","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":null}
- «А какой к этому напиток?» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["drink"],"target_category":"drink","price_intent":null,"meal_structure":null}
- «Что взять к этому из соусов?» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["sauce"],"target_category":"sauce","price_intent":null,"meal_structure":null}
- «А на десерт?» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["dessert"],"target_category":"dessert","price_intent":null,"meal_structure":null}
- «Что взять к этому ещё?» → {"intent":"clarify","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":null}
- «Хочу пиццу с бургером» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["pasta","burger"],"target_category":null,"price_intent":null,"meal_structure":null}
- «Мясное из закусок» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["starter"],"target_category":"starter","price_intent":null,"meal_structure":null}
- «Хочу фастфуд» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":"fastfood_set"}
- «Сытный ужин» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":"full_dinner"}
- «Соберите обед на двоих» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":"full_lunch"}
- «Накормите как следует» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":null,"target_category":null,"price_intent":null,"meal_structure":"full_dinner"}
- «Что-нибудь мясное» → {"intent":"recommend","pairing_drink":null,"occasion":null,"target_categories":["meat","steak","grill"],"target_category":null,"price_intent":null,"meal_structure":null}

Сообщение гостя: {{user_message}}',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin');
