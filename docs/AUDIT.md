# Аудит-лог админских действий

> Статус: дизайн-док. Фронтенд работает на моках, это спецификация бэкенда.

## Зачем

- Видеть, кто менял заказ — чтобы быстро связаться с клиентом, если возник спор.
- Восстановить хронологию изменений меню/тегов/категорий и публикаций промптов.
- Минимальный compliance-след по действиям с прод-данными.

## Сущность `admin_actions`

| поле        | тип             | заметки                                     |
|-------------|-----------------|---------------------------------------------|
| id          | bigserial PK    |                                             |
| admin_id    | uuid (FK users) | NOT NULL                                    |
| target      | text            | enum: `order`, `dish`, `category`, `tag`, `prompt` |
| target_id   | text            | строка: для заказов — UUID, для блюд — id, для промптов — `name` |
| target_label| text            | «человеческое» имя на момент действия (`#1024`, `Том Ям`, `system_main`) — чтобы лента не ломалась после переименований |
| verb        | text            | enum: `create`, `update`, `delete`, `status_change`, `publish`, `rollback` |
| changes     | jsonb           | `[{field, from, to}]` — снимок изменений    |
| created_at  | timestamptz     | DEFAULT now()                               |

Индексы:
- `(admin_id, created_at DESC)` — для «Мои действия»
- `(target, target_id, created_at DESC)` — для истории конкретного заказа/блюда

`changes`-формат:
```json
[
  {"field": "status", "from": "accepted", "to": "cooking"}
]
```

Фронт сам переводит коды (`accepted`→`Принят`) — бэк хранит как есть, без локализации.

## Когда писать

В каждом usecase, который меняет прод-данные. Записываем **в той же транзакции**, что и основное изменение, иначе при сбое можем получить «изменение есть, лога нет» или наоборот.

| usecase                                           | target  | verb            | что в `changes`                              |
|---------------------------------------------------|---------|-----------------|----------------------------------------------|
| `PATCH /admin/orders/{id}/status`                 | order   | status_change   | `status: from→to`                            |
| `PATCH /admin/menu/{id}`                          | dish    | update          | поля: name, price_minor, description, …      |
| `POST /admin/menu`                                | dish    | create          | пусто                                        |
| `DELETE /admin/menu/{id}`                         | dish    | delete          | пусто                                        |
| `POST/PATCH/DELETE /admin/categories[/{id}]`      | category| create/update/delete |                                          |
| `POST/PATCH/DELETE /admin/tags[/{id}]`            | tag     | create/update/delete |                                          |
| `POST /admin/prompts/{name}/publish`              | prompt  | publish         | `version: from→to`                            |
| `POST /admin/prompts/{name}/rollback/{version}`   | prompt  | rollback        | `version: from→to`                            |

`PUT /admin/prompts/{name}/draft` **не логируем** — это личный черновик, не влияет на клиентов.

## Дисамбигуация ФИО

Email админа уникален в `users`. ФИО — нет. На `GET /admin/actions` бэкенд возвращает для каждого действия:

```json
{
  "admin": {
    "id": "uuid",
    "display_name": "Анна Админова",
    "email": "anna.a@demo.local",
    "has_namesake": true
  }
}
```

`has_namesake` — есть ли в `users` другая запись с теми же `first_name`+`last_name`+`role='admin'`. Считаем 1 раз при каждом GET (или, при необходимости, кэшируем на минуту). Фронт по этому флагу решает, показывать email рядом с ФИО или нет.

## Эндпоинты

Все требуют `role='admin'`.

| метод | путь                                | описание                                            |
|-------|-------------------------------------|-----------------------------------------------------|
| GET   | `/admin/actions`                    | лента действий с фильтрами и пагинацией             |
| GET   | `/admin/orders/{id}/actions`        | удобный shortcut: всё, что меняли в этом заказе     |

Фильтры `/admin/actions`:
- `admin_id` (можно `me`)
- `target` (один из enum)
- `from`, `to` (ISO дат)
- `limit` (1..200, default 50), `offset`

Ответ:
```json
{
  "items": [/* AdminAction[] */],
  "total": 412,
  "limit": 50,
  "offset": 0
}
```

## Доступ и приватность

- Любой админ видит ленту любых других админов — это не персональные данные клиентов, а действия сотрудников. Если понадобится — добавим `can_view_all_actions` флаг.
- Логи **не удаляются**. Retention обсудим позже (1 год по умолчанию — реалистично).
- `changes` могут содержать пользовательские данные (адрес доставки и т. п.). Не показываем в логах ничего, что не показывали бы в самом заказе/профиле.

## Будущее (не MVP)

- Поиск по тексту изменений (`changes`-JSONB → `tsvector`).
- Webhook на новое действие — для интеграций (Slack-канал «admin-changes» и т. п.).
- Экспорт ленты в CSV.
