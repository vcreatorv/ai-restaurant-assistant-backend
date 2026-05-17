-- Аудит-лог админских действий: кто, что, когда менял.
--
-- target_id — text, чтобы поддержать и UUID-сущности (orders), и int (dishes/categories/tags),
-- и string-имена (prompts.name). Семантика per-target определяется в usecase.
--
-- target_label — «человеческое» имя на момент действия (#1024, Том Ям, system_main),
-- чтобы лента не ломалась после переименований/удалений.
--
-- changes — JSONB-массив [{field, from, to}]. Бэкенд сохраняет коды/числа как есть,
-- фронт переводит в локализованный текст (например, статус заказа).
--
-- ON DELETE SET NULL admin_id: если админа удалят, лог не пропадёт, но автор станет анонимным.
CREATE TABLE admin_actions (
    id            bigserial   PRIMARY KEY,
    admin_id      uuid        REFERENCES users(id) ON DELETE SET NULL,
    target        text        NOT NULL CHECK (target IN ('order', 'dish', 'category', 'tag', 'prompt')),
    target_id     text        NOT NULL,
    target_label  text        NOT NULL,
    verb          text        NOT NULL CHECK (verb IN ('create', 'update', 'delete', 'status_change', 'publish', 'rollback')),
    changes       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Индекс под «Мои действия» с фильтром по target.
CREATE INDEX admin_actions_admin_created_idx ON admin_actions (admin_id, created_at DESC);

-- Индекс под «История по конкретному объекту» (например, заказу).
CREATE INDEX admin_actions_target_created_idx ON admin_actions (target, target_id, created_at DESC);
