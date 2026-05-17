# Управление промптами через админку

> Статус: дизайн-док, ещё не реализовано. Фронтенд уже работает на моках, это спецификация бэкенда.

## Зачем

Дать админу безопасно менять системные промпты LLM без редеплоя:
- редактирование с черновиком,
- проверка на себе («тестировать в чате» — клиенты получают старую версию, я — новую),
- раскатка на всех с версионированием и быстрым откатом.

## Сущности

### `prompts` (опубликованные версии)

| поле           | тип             | заметки                                  |
|----------------|-----------------|------------------------------------------|
| id             | bigserial PK    |                                          |
| name           | text            | enum: `system_main`, `classification`, `refusal` |
| version        | int             | возрастает в рамках одного `name`        |
| content        | text            | сам промпт                               |
| published_at   | timestamptz     |                                          |
| published_by   | uuid (FK users) |                                          |

UNIQUE(`name`, `version`). Активная версия по `name` — та, что с максимальным `version`.

### `prompt_drafts` (черновики, по одному на админа на промпт)

| поле        | тип             | заметки                            |
|-------------|-----------------|------------------------------------|
| admin_id    | uuid (FK users) |                                    |
| prompt_name | text            |                                    |
| content     | text            |                                    |
| updated_at  | timestamptz     |                                    |

PK (`admin_id`, `prompt_name`). При выходе админа черновик не удаляется.

## Логика выбора промпта в чате

```
SELECT COALESCE(d.content, p.content)
FROM prompts p
LEFT JOIN prompt_drafts d
       ON d.prompt_name = p.name
      AND d.admin_id    = $current_user_id  -- NULL, если не админ
WHERE p.name = $name
  AND p.version = (SELECT MAX(version) FROM prompts WHERE name = p.name);
```

Кэшируем активные версии в памяти процесса (map by name → content+version), invalidate по `prompt_name` при `PUT /admin/prompts/{name}/publish`. Для draft-выборки кэш не нужен (берём из БД, нагрузка от админов малая).

## Эндпоинты

Все требуют `role='admin'`.

| метод | путь                                       | описание                                              |
|-------|--------------------------------------------|-------------------------------------------------------|
| GET   | `/admin/prompts`                           | список промптов с активной версией и (если есть) моим черновиком |
| GET   | `/admin/prompts/{name}`                    | активная версия + мой черновик + история              |
| PUT   | `/admin/prompts/{name}/draft`              | создать/обновить мой черновик: `{content}`            |
| DELETE| `/admin/prompts/{name}/draft`              | удалить мой черновик                                  |
| POST  | `/admin/prompts/{name}/publish`            | опубликовать draft как новую версию (атомарно)        |
| POST  | `/admin/prompts/{name}/rollback/{version}` | сделать активной указанную старую версию (новая запись с тем же content) |

### Валидация (на сохранении draft и publish)

- `len(content) ∈ [50, 8000]`;
- все обязательные плейсхолдеры присутствуют (зависит от `name`):
  - `system_main`: `{{user_allergens}}`, `{{user_dietary}}`, `{{dish_list}}`
  - `classification`: `{{user_message}}`
  - `refusal`: нет
- неизвестные `{{...}}` — варн (вернуть в `details.warnings[]`, не блокировать).

Ответ при ошибке валидации — стандартный `error.code = "prompt_invalid"`, `details.fields[]`.

## Миграция

Сидаем 1-ю версию каждого промпта дефолтным содержимым из `internal/prompts/defaults.go` (или похожего места). Это replace для текущего «зашитого в код» промпта.

## Безопасность

- Только `role='admin'`.
- Логировать все `PUT /draft`, `DELETE /draft`, `POST /publish`, `POST /rollback` через `admin_actions` (см. `AUDIT.md`).
- На `publish` — отдельный `audit.action = 'prompt.publish'` с diff'ом (старая версия → новая).

## Будущее (не MVP)

- Превью без раскатки: `POST /admin/prompts/{name}/preview {prompt_content, user_message}` — синхронный вызов LLM с этим промптом, без записи в БД и без логов разговоров. Полезно, чтобы быстро проверить пару вариантов до сохранения draft.
- Канареечная раскатка: процент трафика на новую версию. Сильно сложнее, делать только если правда понадобится.
- A/B-тест двух промптов с метриками конверсии.
