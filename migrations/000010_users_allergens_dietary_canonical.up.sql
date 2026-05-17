-- Приведение users.allergens и users.dietary к каноническим английским кодам
-- (тем же, что используются в dishes.allergens / dishes.dietary).
--
-- Причина: ранее фронт сохранял в профиль русские строки («орехи», «арахис»),
-- а в Qdrant payload и в dishes.allergens лежат коды («nuts», «peanuts»).
-- Из-за рассогласования must_not allergens=<rus> в Qdrant ничего не фильтровал —
-- блюда с орехами/арахисом доходили до LLM, ассистент их рекомендовал.
--
-- После миграции данные на одном языке во всех слоях, плюс CHECK гарантирует,
-- что любая будущая запись вне whitelist'а отвергнется ещё на уровне БД.

-- 1. Конвертируем существующие значения.
UPDATE users
SET allergens = (
    SELECT COALESCE(array_agg(DISTINCT mapped), '{}'::text[])
    FROM (
        SELECT CASE lower(a)
            WHEN 'орехи'        THEN 'nuts'
            WHEN 'арахис'       THEN 'peanuts'
            WHEN 'лактоза'      THEN 'dairy'
            WHEN 'молоко'       THEN 'dairy'
            WHEN 'глютен'       THEN 'gluten'
            WHEN 'морепродукты' THEN 'shellfish'
            WHEN 'яйцо'         THEN 'eggs'
            WHEN 'яйца'         THEN 'eggs'
            WHEN 'соя'          THEN 'soy'
            WHEN 'кунжут'       THEN 'sesame'
            WHEN 'сельдерей'    THEN 'celery'
            WHEN 'горчица'      THEN 'mustard'
            WHEN 'рыба'         THEN 'fish'
            ELSE a -- уже код или неизвестное значение — оставим, CHECK ниже отсеет невалидное
        END AS mapped
        FROM unnest(allergens) AS a
    ) t
    WHERE mapped IN ('celery','dairy','eggs','fish','gluten','mustard','nuts','peanuts','sesame','shellfish','soy')
)
WHERE allergens <> '{}';

UPDATE users
SET dietary = (
    SELECT COALESCE(array_agg(DISTINCT mapped), '{}'::text[])
    FROM (
        SELECT CASE lower(d)
            WHEN 'вегетарианство'   THEN 'vegetarian'
            WHEN 'вегетарианец'     THEN 'vegetarian'
            WHEN 'веган'            THEN 'vegan'
            WHEN 'веганство'        THEN 'vegan'
            WHEN 'халяль'           THEN 'halal'
            WHEN 'кошер'            THEN 'kosher'
            WHEN 'без глютена'      THEN 'gluten_free'
            WHEN 'без лактозы'      THEN 'lactose_free'
            ELSE d
        END AS mapped
        FROM unnest(dietary) AS d
    ) t
    WHERE mapped IN ('vegetarian','vegan','halal','kosher','gluten_free','lactose_free')
)
WHERE dietary <> '{}';

-- 2. Гарантия консистентности: CHECK ловит любые будущие невалидные значения
--    раньше, чем они окажутся в Qdrant-фильтре и сломают безопасность.
ALTER TABLE users
    ADD CONSTRAINT users_allergens_whitelist
    CHECK (allergens <@ ARRAY['celery','dairy','eggs','fish','gluten','mustard','nuts','peanuts','sesame','shellfish','soy']::text[]);

ALTER TABLE users
    ADD CONSTRAINT users_dietary_whitelist
    CHECK (dietary <@ ARRAY['vegetarian','vegan','halal','kosher','gluten_free','lactose_free']::text[]);
