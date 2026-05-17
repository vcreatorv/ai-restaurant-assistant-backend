-- Версионируемые системные промпты для LLM.
-- Активная версия по `name` — та, у которой максимальный `version`.
CREATE TABLE prompts (
    id            bigserial   PRIMARY KEY,
    name          text        NOT NULL,
    version       integer     NOT NULL,
    content       text        NOT NULL,
    published_at  timestamptz NOT NULL DEFAULT now(),
    published_by  uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (name, version),
    CHECK (length(content) BETWEEN 50 AND 8000)
);

CREATE INDEX prompts_name_version_idx ON prompts (name, version DESC);

-- Личный черновик админа на промпт. Один черновик на админа на name.
-- При входящем сообщении в чате, если автор — админ и под ним есть черновик
-- на запрашиваемый prompt_name, usecase подставляет content из черновика
-- вместо опубликованной версии.
CREATE TABLE prompt_drafts (
    admin_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prompt_name  text        NOT NULL,
    content      text        NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (admin_id, prompt_name),
    CHECK (length(content) BETWEEN 50 AND 8000)
);

CREATE INDEX prompt_drafts_admin_idx ON prompt_drafts (admin_id);

-- Засеваем дефолтный system_main, чтобы чат не падал на пустой таблице.
-- Контент специально задан минимально безопасный: после первой раскатки
-- из админки заменится осмысленным.
INSERT INTO prompts (name, version, content, published_by)
SELECT
    'system_main',
    1,
    'Ты — официант ресторана. Помогай гостям выбирать блюда из предоставленного меню. Не выдумывай блюда вне списка. В конце ответа всегда добавляй блок ```json {"recommended_dish_ids":[<id>]}``` со списком id рекомендованных блюд.',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin');
