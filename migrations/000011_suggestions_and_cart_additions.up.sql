-- Добавляем две новые сущности под админ-аналитику:
--
-- 1) chat_suggestions — пресет-подсказки внизу чата ("Что-то острое и лёгкое"
--    и т.п.), редактируемые админом. Раньше были захардкожены во фронте.
--    clicks_count держим прямо в строке для дешёвой аналитики «как часто клику-
--    ют по подсказке»; ивент-таблица под клики не нужна, пока не понадобятся
--    временные ряды по конкретной подсказке.
--
-- 2) cart_additions — событийный лог добавлений блюд в корзину. Нужен, чтобы
--    отдельно считать «в корзину из чата» и «из меню». В cart_items state-таблица,
--    она не хранит историю добавлений (одно блюдо может быть добавлено/убрано
--    несколько раз → один state, но N событий).

CREATE TABLE chat_suggestions (
    id           bigserial   PRIMARY KEY,
    text         text        NOT NULL CHECK (length(text) BETWEEN 1 AND 80),
    sort_order   integer     NOT NULL DEFAULT 0,
    is_active    boolean     NOT NULL DEFAULT true,
    clicks_count bigint      NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX chat_suggestions_active_sort_idx
    ON chat_suggestions (is_active, sort_order, id);

-- Стартовый набор подсказок (тот же, что раньше был во фронте).
INSERT INTO chat_suggestions (text, sort_order) VALUES
    ('Что-то острое и лёгкое',  1),
    ('Хочу сытное на двоих',    2),
    ('Подойдёт под белое вино', 3),
    ('До 500 рублей',           4);

-- Источник добавления в корзину:
--   'chat'  — гость нажал «+» на карточке блюда в ответе ассистента;
--   'menu'  — добавление со страницы /menu;
--   'cart'  — изменение количества в самой корзине (на будущее);
--   'other' — fallback для старых клиентов, не передающих source.
-- message_id заполняется только когда source='chat' и фронт прислал id
-- assistant-сообщения, из которого пользователь взял блюдо.
CREATE TABLE cart_additions (
    id          bigserial   PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dish_id     integer     NOT NULL REFERENCES dishes(id) ON DELETE RESTRICT,
    quantity    integer     NOT NULL CHECK (quantity > 0 AND quantity <= 50),
    source      text        NOT NULL CHECK (source IN ('chat', 'menu', 'cart', 'other')),
    message_id  uuid        REFERENCES chat_messages(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cart_additions_user_created_idx
    ON cart_additions (user_id, created_at DESC);

CREATE INDEX cart_additions_source_created_idx
    ON cart_additions (source, created_at DESC);

CREATE INDEX cart_additions_message_idx
    ON cart_additions (message_id) WHERE message_id IS NOT NULL;
