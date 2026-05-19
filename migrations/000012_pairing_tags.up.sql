-- Pairing-теги для семантического обогащения embed-текста блюда.
--
-- Зачем: «голый» эмбеддинг блюда (имя + описание + состав + кухня + категория)
-- плохо матчит запросы по intent'у — «под белое вино», «на свидание», «лёгкий
-- перекус», «накормите как следует». Cohere multilingual-v3 хорошо различает
-- семантику, но если в embed-тексте нет токенов «белому вину» — никакой пейринг
-- этой парой запрос-блюдо не возникнет, retrieval вытаскивает по лексическому
-- overlap'у (запрос «белое вино» → «крем-суп грибной» из-за «белых грибов»).
--
-- Решение: контролируемая vocabulary тегов в БД, M2M с блюдами, и блок «Подходит к: …
-- / Подаётся как: … / Хорошо для: … / Тип: …» в конце embed-текста — каждый
-- тег вносит свою короткую фразу (embed_phrase). Vocabulary засевается миграцией;
-- править значения позже можно через админ-API (фаза 2). После изменения
-- vocabulary или назначений тегов блюдо нужно реиндексировать.
--
-- Оси (axis):
--   drink     — с каким напитком сочетается (pair_white_wine, pair_red_wine, …)
--   occasion  — для какого повода (occasion_date, occasion_kids, …)
--   role      — слот в трапезе (role_aperitif, role_main, role_finish, …)
--   vibe      — настроение / температура / плотность (vibe_warming, vibe_light, …)

CREATE TABLE pairing_tags (
    slug         text        PRIMARY KEY,
    axis         text        NOT NULL CHECK (axis IN ('drink', 'occasion', 'role', 'vibe')),
    label        text        NOT NULL,
    embed_phrase text        NOT NULL,
    sort_order   int         NOT NULL DEFAULT 100,
    is_active    boolean     NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Индекс под выдачу активных тегов админ-UI'у, сгруппированных по оси.
CREATE INDEX pairing_tags_axis_sort_idx
    ON pairing_tags (axis, sort_order)
    WHERE is_active;

CREATE TABLE dish_pairing_tags (
    dish_id  int  NOT NULL REFERENCES dishes(id)        ON DELETE CASCADE,
    tag_slug text NOT NULL REFERENCES pairing_tags(slug) ON DELETE CASCADE,
    PRIMARY KEY (dish_id, tag_slug)
);

-- Обратный индекс: «какие блюда помечены этим тегом» (для каскадного
-- реиндекса при изменении embed_phrase у конкретного тега).
CREATE INDEX dish_pairing_tags_tag_slug_idx
    ON dish_pairing_tags (tag_slug);

-- ── Засев vocabulary (22 тега, 4 оси) ────────────────────────────────────────
-- Slug фиксируется: на него ссылается dish_pairing_tags.tag_slug, а также
-- админ-API при валидации входящих списков.
-- ON CONFLICT DO NOTHING — миграция идемпотентна (на случай ручного засева).

INSERT INTO pairing_tags (slug, axis, label, embed_phrase, sort_order) VALUES
    -- drink (10): пейринг с напитками
    ('pair_white_wine',  'drink', 'к белому вину',           'белому вину',                 10),
    ('pair_red_wine',    'drink', 'к красному вину',         'красному вину',               20),
    ('pair_sparkling',   'drink', 'к игристому',             'игристому, шампанскому',      30),
    ('pair_beer_light',  'drink', 'к светлому пиву',         'светлому пиву, лагеру',       40),
    ('pair_beer_dark',   'drink', 'к тёмному пиву',          'тёмному пиву, стауту',        50),
    ('pair_cider',       'drink', 'к сидру',                 'сидру',                       60),
    ('pair_cocktails',   'drink', 'к коктейлям',             'коктейлям, аперитивам',       70),
    ('pair_coffee',      'drink', 'к кофе',                  'кофе',                        80),
    ('pair_tea',         'drink', 'к чаю',                   'чаю',                         90),
    ('pair_lemonade',    'drink', 'к лимонаду',              'лимонаду, морсу',            100),

    -- occasion (5): повод / тип мероприятия
    ('occasion_date',           'occasion', 'свидание',          'романтического ужина, свидания',          10),
    ('occasion_celebration',    'occasion', 'праздник',          'праздничного ужина, торжества',           20),
    ('occasion_business_lunch', 'occasion', 'деловой обед',      'делового обеда',                          30),
    ('occasion_kids',           'occasion', 'для детей',         'детей, без острого и алкоголя',           40),
    ('occasion_breakfast',      'occasion', 'завтрак',           'завтрака',                                50),

    -- role (7): слот блюда в трапезе
    ('role_aperitif',     'role', 'аперитив',           'закуска перед основным, аперитив', 10),
    ('role_starter',      'role', 'лёгкий старт',       'лёгкий старт, первое блюдо к столу', 20),
    ('role_main',         'role', 'основное горячее',   'основное горячее',                   30),
    ('role_main_filling', 'role', 'сытное основное',    'сытное основное, наесться',          40),
    ('role_share',        'role', 'на компанию',        'на компанию, разделить',             50),
    ('role_side',         'role', 'гарнир',             'гарнир к мясу или рыбе',             60),
    ('role_finish',       'role', 'финал ужина',        'финал ужина, десерт',                70),

    -- vibe (4): плотность / температура / настроение
    ('vibe_warming',     'vibe', 'согревает',          'согревает в холодную погоду',         10),
    ('vibe_refreshing',  'vibe', 'освежает',           'освежает в жару',                     20),
    ('vibe_light',       'vibe', 'лёгкое',             'лёгкое, не тяжёлое',                  30),
    ('vibe_hearty',      'vibe', 'сытное',             'сытное, плотное',                     40)
ON CONFLICT (slug) DO NOTHING;
