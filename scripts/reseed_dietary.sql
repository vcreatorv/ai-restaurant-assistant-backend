-- Переразметка dietary-тегов по всем 193 блюдам.
--
-- ПОЧЕМУ: в seed-данных dietary были расставлены массово (большинству блюд
-- проставлено [vegetarian, vegan, gluten_free, lactose_free] независимо от
-- реального состава). В результате фильтр Qdrant `must=dietary=vegan`
-- честно возвращал Топ Сирлойн как vegan-блюдо.
--
-- ПРАВИЛА:
--   vegan        — никаких животных продуктов (мясо/рыба/яйца/молоко/мёд)
--   vegetarian   — нет мяса и рыпы (молоко, яйца, мёд допустимы)
--   gluten_free  — без пшеницы/ржи/ячменя/овса (исключает хлеб, пиво, обычную пасту)
--   lactose_free — без молочных продуктов
--   halal        — без свинины и алкоголя; для блюд с явным "halal" в названии — выставлено
--                  Для остальных мясных оставлено пусто (нет гарантии способа закланивания).
--
-- ДЕЛАЕТ: переписывает поле dishes.dietary одним UPDATE через FROM (VALUES …).
-- Идемпотентно — повторный запуск приводит к тому же финальному состоянию.
-- После применения нужно `make embed-menu`, чтобы payload в Qdrant обновился
-- (поле dietary в Qdrant payload смотрит на старое состояние до реиндекса).

BEGIN;

UPDATE dishes
SET dietary = data.tags
FROM (VALUES
    -- ── Закуски ────────────────────────────────────────────────────────────
    (1,  ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),  -- Бабагануш (овощной, тахини, кедровые)
    (2,  ARRAY['vegan','vegetarian','lactose_free','halal']),                -- Баклажаны в панировке (gluten в панировке)
    (3,  ARRAY[]::text[]),                                                    -- Вителло Тоннато (говядина + тунцовый соус)
    (4,  ARRAY[]::text[]),                                                    -- Карпаччо Денвер (говядина)
    (5,  ARRAY[]::text[]),                                                    -- Креветки в кляре (рыба)
    (6,  ARRAY[]::text[]),                                                    -- Куриные крылья
    (7,  ARRAY[]::text[]),                                                    -- Куриные стрипсы
    (8,  ARRAY[]::text[]),                                                    -- Мясное плато
    (9,  ARRAY['vegetarian','gluten_free','halal']),                          -- Сырное плато
    (10, ARRAY[]::text[]),                                                    -- Тартар авокадо с креветками
    (11, ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Чесночные гренки (хлеб → gluten)
    (57, ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Фокачча с розмарином
    (58, ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Фокачча с томатами
    (59, ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Хлебная корзина
    (76, ARRAY[]::text[]),                                                    -- Тартар из говядины
    (173,ARRAY[]::text[]),                                                    -- Ролл Филадельфия (лосось + сыр)
    (174,ARRAY[]::text[]),                                                    -- Ролл Калифорния (краб)
    (182,ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Хумус с овощами и питой
    (189,ARRAY[]::text[]),                                                    -- Большое мясное плато
    (190,ARRAY['vegetarian','gluten_free','halal']),                          -- Сырная доска
    (193,ARRAY[]::text[]),                                                    -- Закусочный сет (крылья + гренки)

    -- ── Салаты ─────────────────────────────────────────────────────────────
    (12, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Витаминный (капуста, морковь)
    (13, ARRAY['vegetarian','gluten_free','halal']),                          -- Греческий (брынза)
    (14, ARRAY[]::text[]),                                                    -- Пастрами (мясо)
    (15, ARRAY['vegetarian']),                                                -- Буррата (моцарелла + терияки с глютеном)
    (16, ARRAY[]::text[]),                                                    -- Бык-тейсти (фарш)
    (17, ARRAY['vegetarian','gluten_free','lactose_free','halal']),           -- Зелёный (мёд → не vegan)
    (18, ARRAY['vegetarian','gluten_free','halal']),                          -- Капрезе (моцарелла + песто)
    (19, ARRAY[]::text[]),                                                    -- С говяжьим языком
    (20, ARRAY[]::text[]),                                                    -- С креветками
    (21, ARRAY['vegetarian','gluten_free','halal']),                          -- С печёной тыквой и фетой
    (22, ARRAY[]::text[]),                                                    -- С ростбифом
    (23, ARRAY['vegetarian','gluten_free','halal']),                          -- Томаты со страчателлой
    (24, ARRAY[]::text[]),                                                    -- Цезарь с креветками
    (25, ARRAY[]::text[]),                                                    -- Цезарь с курицей
    (26, ARRAY[]::text[]),                                                    -- Стейк-салат
    (27, ARRAY[]::text[]),                                                    -- Тёплый с цыплёнком
    (179,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Боул с киноа и авокадо

    -- ── Супы ───────────────────────────────────────────────────────────────
    (28, ARRAY[]::text[]),                                                    -- Бульон куриный
    (29, ARRAY[]::text[]),                                                    -- Борщ (говядина + сметана)
    (30, ARRAY['vegetarian','halal']),                                        -- Крем-суп грибной (сливки + багет)
    (31, ARRAY[]::text[]),                                                    -- Крем-суп сырный (копчёный цыплёнок)
    (32, ARRAY[]::text[]),                                                    -- Том-ям (морепродукты)
    (170,ARRAY[]::text[]),                                                    -- Том-ям с креветками
    (171,ARRAY['vegetarian']),                                                -- Мисо-суп с тофу (даси/мисо часто с алк/глютеном)

    -- ── Пицца и паста ──────────────────────────────────────────────────────
    (33, ARRAY[]::text[]),                                                    -- Пицца Баварская (чоризо/бекон)
    (34, ARRAY['vegetarian','halal']),                                        -- Пицца Маргарита
    (35, ARRAY['vegetarian','halal']),                                        -- Пицца Сырная (4 сыра)
    (36, ARRAY[]::text[]),                                                    -- Пицца Цезарь (курица)
    (37, ARRAY[]::text[]),                                                    -- Пицца Чоризо
    (62, ARRAY[]::text[]),                                                    -- Паста болоньезе (фарш)
    (63, ARRAY[]::text[]),                                                    -- Паста с томлёной щекой (мясо)
    (176,ARRAY[]::text[]),                                                    -- Карбонара (гуанчале — свинина)
    (177,ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Аррабиата (без сыра, чили + чеснок)
    (178,ARRAY['vegetarian','gluten_free']),                                  -- Ризотто с белыми грибами (пармезан + белое вино)

    -- ── Бургеры (все мясные/морепродукты) ──────────────────────────────────
    (86, ARRAY[]::text[]),  -- JD (томлёная говядина + бурбон, не halal)
    (87, ARRAY[]::text[]),  -- THE БЫК (Блэк Ангус)
    (88, ARRAY[]::text[]),  -- Вишнёвый бизон (бекон)
    (89, ARRAY[]::text[]),  -- С креветками
    (90, ARRAY[]::text[]),  -- Чизбургер

    -- ── Стейки (все говядина/свинина) ──────────────────────────────────────
    (64, ARRAY[]::text[]),  (67, ARRAY[]::text[]),  (69, ARRAY[]::text[]),
    (70, ARRAY[]::text[]),  (72, ARRAY[]::text[]),  (73, ARRAY[]::text[]),
    (74, ARRAY[]::text[]),  (75, ARRAY[]::text[]),  (81, ARRAY[]::text[]),
    (82, ARRAY[]::text[]),  (83, ARRAY[]::text[]),  (84, ARRAY[]::text[]),
    (85, ARRAY[]::text[]),  (192,ARRAY[]::text[]),

    -- ── На гриле (все мясные, кроме явно halal) ────────────────────────────
    (38, ARRAY[]::text[]),  (39, ARRAY[]::text[]),  (40, ARRAY[]::text[]),
    (41, ARRAY[]::text[]),  (42, ARRAY[]::text[]),  (43, ARRAY[]::text[]),
    (44, ARRAY[]::text[]),  (45, ARRAY[]::text[]),  (46, ARRAY[]::text[]),
    (47, ARRAY[]::text[]),  (48, ARRAY[]::text[]),  (49, ARRAY[]::text[]),
    (183,ARRAY['halal']),                                                      -- Шашлык ягнёнка халяль (явно)
    (184,ARRAY[]::text[]),                                                    -- Куриный sriracha

    -- ── Мясное ─────────────────────────────────────────────────────────────
    (50, ARRAY[]::text[]),  (51, ARRAY[]::text[]),  (52, ARRAY[]::text[]),
    (53, ARRAY[]::text[]),  (54, ARRAY[]::text[]),  (55, ARRAY[]::text[]),
    (56, ARRAY[]::text[]),  (65, ARRAY[]::text[]),  (66, ARRAY[]::text[]),
    (78, ARRAY[]::text[]),  (79, ARRAY[]::text[]),  (80, ARRAY[]::text[]),
    (172,ARRAY[]::text[]),  (175,ARRAY[]::text[]),
    (180,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Овощное рагу (рататуй)

    -- ── Морепродукты (все рыба/морепродукты) ───────────────────────────────
    (60, ARRAY[]::text[]),  (61, ARRAY[]::text[]),  (68, ARRAY[]::text[]),
    (71, ARRAY[]::text[]),  (77, ARRAY[]::text[]),  (185,ARRAY[]::text[]),
    (191,ARRAY[]::text[]),

    -- ── Гарниры ────────────────────────────────────────────────────────────
    (91, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Брокколи
    (92, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Картофель батат фри
    (93, ARRAY['vegetarian','halal']),                                        -- Диппер (майонез + JD-соус, не halal)
    (94, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Картофель по-деревенски
    (95, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Картофель фри
    (96, ARRAY['vegetarian','gluten_free','halal']),                          -- Картофель с пармезаном/трюфель
    (97, ARRAY['vegetarian','gluten_free','halal']),                          -- Картофельное пюре (молоко/масло)
    (98, ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Овощи гриль
    (99, ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Рис с овощами wok (соевый соус с глютеном)

    -- ── Соусы ──────────────────────────────────────────────────────────────
    (100,ARRAY['vegetarian','gluten_free','lactose_free','halal']),           -- 1000 островов (майонез)
    (101,ARRAY['vegetarian','gluten_free']),                                  -- JD (бурбон, не halal)
    (102,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Барбекю
    (103,ARRAY['vegetarian','gluten_free','halal']),                          -- Блю чиз
    (104,ARRAY['vegetarian','gluten_free','halal']),                          -- Грибной (сливки)
    (105,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Кетчуп
    (106,ARRAY['vegetarian','gluten_free','lactose_free','halal']),           -- Медово-горчичный (мёд → не vegan)
    (107,ARRAY['vegetarian','gluten_free','halal']),                          -- Перечный (сливочный)
    (108,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Сальса
    (109,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Сладкий чили
    (110,ARRAY['vegetarian','gluten_free','halal']),                          -- Black Pepper (сливочный)
    (111,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Брусничный с розмарином

    -- ── Десерты ────────────────────────────────────────────────────────────
    (112,ARRAY['vegetarian','halal']),                                        -- Медовик (сметана + мёд)
    (113,ARRAY['vegetarian','gluten_free','halal']),                          -- Мороженое
    (114,ARRAY['vegetarian','gluten_free','halal']),                          -- Панна-Котта (сливки)
    (115,ARRAY['vegetarian','gluten_free','halal']),                          -- Фисташковый рулет (меренга — без муки)
    (116,ARRAY['vegetarian','halal']),                                        -- Чизкейк баскский (песочная основа)
    (117,ARRAY['vegetarian','halal']),                                        -- Шоколадный Фондан
    (181,ARRAY['vegan','vegetarian','lactose_free','halal']),                 -- Гранола (овсянка → не gluten_free; кокос. молоко)
    (186,ARRAY['vegetarian','halal']),                                        -- Чизкейк НЙ
    (187,ARRAY['vegetarian']),                                                -- Тирамису (часто с алкоголем)
    (188,ARRAY['vegetarian','halal']),                                        -- Эклер

    -- ── Напитки — кофейная карта ───────────────────────────────────────────
    (118,ARRAY['vegetarian','gluten_free','halal']),                          -- Айс Латте Карамель-Бузина (молоко)
    (119,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Американо
    (120,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Бамбл апельсин
    (121,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Бамбл гранат
    (122,ARRAY['vegetarian','gluten_free','halal']),                          -- Гляссе (мороженое)
    (123,ARRAY['vegetarian','gluten_free','halal']),                          -- Горячий шоколад (молоко)
    (124,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Двойной эспрессо
    (125,ARRAY['vegetarian','gluten_free','halal']),                          -- Капучино (молоко)
    (126,ARRAY['vegetarian','gluten_free','halal']),                          -- Латте
    (127,ARRAY['vegetarian','gluten_free','halal']),                          -- Раф (сливки)
    (128,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Фильтр-кофе
    (129,ARRAY['vegetarian','gluten_free','halal']),                          -- Флэт Уайт (молоко)
    (130,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Эспрессо
    (131,ARRAY['vegetarian','gluten_free']),                                  -- Какао с маршмелоу (желатин → halal под вопросом)
    (132,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Капучино на альт. молоке
    (133,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Латте на альт. молоке

    -- ── Напитки — чай ──────────────────────────────────────────────────────
    (134,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Ассам
    (135,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Гречишный
    (136,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Жасминовый
    (137,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Манго & Имбирь
    (138,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Мандарин & Чёрная смородина
    (139,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Молочный улун (сам чай без молока)
    (140,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Облепиха & Груша
    (141,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Ромашковый
    (142,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Сенча
    (143,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Таёжный
    (144,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Фруктовый пунш
    (145,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Цейлонский с чабрецом

    -- ── Напитки — лимонады и Б/А коктейли ──────────────────────────────────
    (146,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Б/А №1
    (147,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Вишня & Миндаль
    (148,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Кола
    (149,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Крем Сливки (сливочный аромат)
    (150,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Малина&Маракуйя
    (151,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Тархун
    (152,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Щавель Клубника
    (153,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Яблоко-киви
    (154,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Свежий Мандарин
    (155,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Б/А №2
    (156,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Б/А №3
    (157,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Лимонад Лесная ягода
    (158,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Глинтвейн б/а
    (168,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Апероль №1 б/а
    (169,ARRAY['vegan','vegetarian','gluten_free','lactose_free','halal']),   -- Апероль №2 б/а

    -- ── Алкоголь (б/а вина и пиво — halal не указываем для строгости) ─────
    (159,ARRAY['vegan','vegetarian','gluten_free','lactose_free']),           -- Б/А Albali Red Cabernet
    (160,ARRAY['vegan','vegetarian','gluten_free','lactose_free']),           -- Б/А Albali White Sparkling
    (161,ARRAY['vegan','vegetarian','gluten_free','lactose_free']),           -- Б/А Riesling
    (162,ARRAY['vegan','vegetarian','lactose_free']),                         -- Б/А Pabst (пиво → glуten)
    (163,ARRAY['vegan','vegetarian','lactose_free']),                         -- Вишнёвый Эль THE Бык
    (164,ARRAY['vegan','vegetarian','lactose_free']),                         -- Нефильтрованное THE Бык
    (165,ARRAY['vegan','vegetarian','gluten_free','lactose_free']),           -- Сидр THE Бык (яблочный)
    (166,ARRAY['vegan','vegetarian','lactose_free']),                         -- Тёмное THE Бык
    (167,ARRAY['vegan','vegetarian','lactose_free'])                          -- Ячменный лагер THE Бык
) AS data(dish_id, tags)
WHERE dishes.id = data.dish_id;

COMMIT;

-- Контроль: сколько каких dietary, и есть ли блюда без dietary вообще (мясные)
SELECT
    'vegan'        AS tag, COUNT(*) FROM dishes WHERE 'vegan'        = ANY(dietary)
UNION ALL SELECT 'vegetarian',   COUNT(*) FROM dishes WHERE 'vegetarian'   = ANY(dietary)
UNION ALL SELECT 'gluten_free',  COUNT(*) FROM dishes WHERE 'gluten_free'  = ANY(dietary)
UNION ALL SELECT 'lactose_free', COUNT(*) FROM dishes WHERE 'lactose_free' = ANY(dietary)
UNION ALL SELECT 'halal',        COUNT(*) FROM dishes WHERE 'halal'        = ANY(dietary)
UNION ALL SELECT '(none)',       COUNT(*) FROM dishes WHERE COALESCE(array_length(dietary, 1), 0) = 0;

-- Sanity: убедимся, что среди vegan нет очевидных мясных категорий
SELECT d.id, d.name, c.name AS category
FROM dishes d JOIN categories c ON c.id = d.category_id
WHERE 'vegan' = ANY(d.dietary)
  AND c.name IN ('Стейки','Бургеры','Мясное','Морепродукты','На гриле')
ORDER BY c.name, d.id;
