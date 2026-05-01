# Ручное тестирование auth-флоу

Минимальный runbook для прогона `auth + profile` end-to-end через Postman / curl.

> **Поддержка коллекции — обязательное правило проекта.**
> При добавлении или изменении любого HTTP-эндпоинта или поведения, которое
> можно увидеть «снаружи» (новый фильтр в каталоге, новое поле в профиле,
> новое правило в LLM-prompt'е, новые валидации и т.д.) **в этом же PR**:
>
> 1. добавить или обновить запрос в `postman/ai-restaurant-assistant.postman_collection.json`
>    (новый item в подходящей папке, с тестами в `event.test`),
> 2. добавить или обновить шаг в `POSTMAN.md` с ожидаемыми кодами/телами и пояснением «что валидируется»,
> 3. при изменении seed-данных меню — пометить, что нужно перезапустить `make seed && make embed-menu`.
>
> Цель: всегда иметь готовый набор ручек, на которых можно за 5 минут
> вручную убедиться, что новая функциональность работает и не сломала
> старое.

## Подготовка

1. Файлы конфига уже готовы:
   - `.env` — секреты (PG/Redis пароли, Qdrant key); значения сгенерированы для локального dev.
   - `configs/config.yaml` — не-секретные настройки (HTTP addr, log level, session TTL, bcrypt cost).
2. Поднять стек:
   ```sh
   make docker-up
   make docker-logs   # = docker compose logs -f app
   ```
   `migrate` применит `000001_users.up.sql` один раз; `app` поднимется на `http://localhost:8081` (внешний порт из `.env::HTTP_PORT`; внутри контейнера сервис слушает `:8080`).
3. В Postman создать коллекцию с базовым URL `http://localhost:8081/api/v1`.
4. Включить **cookie jar** (Postman делает это по умолчанию для домена; убедиться, что cookies автосохраняются между запросами).

## Pre-request скрипт для CSRF

В коллекции в Pre-request Script положить:

```js
const csrf = pm.collectionVariables.get("csrf");
if (csrf) {
    pm.request.headers.upsert({ key: "X-CSRF-Token", value: csrf });
}
```

В Tests-скрипте каждого ответа, который возвращает `SessionInfo` (`/auth/session`, `/auth/register`, `/auth/login`):

```js
const body = pm.response.json();
if (body && body.csrf) {
    pm.collectionVariables.set("csrf", body.csrf);
}
```

После этого CSRF подхватывается автоматически.

## Прогон

### 1. Bootstrap — получить cookie + CSRF

```
GET /api/v1/auth/session
```

Ожидание: `200`, body:
```json
{
  "is_guest": true,
  "csrf": "...",
  "user_id": null,
  "role": null
}
```

В response — `Set-Cookie: session_id=...; HttpOnly; Path=/`. В коллекции CSRF сохранён.

### 2. Регистрация

```
POST /api/v1/auth/register
Headers: X-CSRF-Token (auto), Content-Type: application/json
Body: { "email": "test@example.com", "password": "secret123" }
```

Ожидание: `200`, body — обновлённый `SessionInfo`:
```json
{
  "is_guest": false,
  "csrf": "<новый, ротированный>",
  "user_id": "<uuid>",
  "role": "customer"
}
```

CSRF в коллекции автоматически обновится через Tests-скрипт.

### 3. Подтвердить session-state

```
GET /api/v1/auth/session
```

Ожидание: `200`, `is_guest=false`, `role=customer`.

### 4. Получить профиль

```
GET /api/v1/profile
```

Ожидание: `200`:
```json
{
  "email": "test@example.com",
  "first_name": null,
  "last_name": null,
  "phone": null,
  "allergens": [],
  "dietary": []
}
```

### 5. Обновить профиль

```
PATCH /api/v1/profile
Headers: X-CSRF-Token (auto)
Body: {
  "first_name": "Иван",
  "last_name": "Петров",
  "phone": "+79001234567",
  "allergens": ["shellfish"],
  "dietary": ["vegan"]
}
```

Ожидание: `200`, профиль с заполненными полями.

### 6. Сменить пароль

```
PATCH /api/v1/auth/password
Headers: X-CSRF-Token (auto)
Body: { "current_password": "secret123", "new_password": "newsecret456" }
```

Ожидание: `204` без тела.

### 7. Выход

```
POST /api/v1/auth/logout
Headers: X-CSRF-Token (auto)
```

Ожидание: `204`. Сессия в Redis удалена.

### 8. Проверить, что мы стали гостем

```
GET /api/v1/auth/session
```

Ожидание: `200`, `is_guest=true`, `user_id=null`. Создаётся новая Redis-сессия, в коллекции свежий CSRF.

### 9. Логин с новым паролем

```
POST /api/v1/auth/login
Body: { "email": "test@example.com", "password": "newsecret456" }
```

Ожидание: `200`, `SessionInfo` с `role=customer`, `user_id=<тот же>`. CSRF снова ротирован.

### 10. Логин с неверным паролем

```
POST /api/v1/auth/login
Body: { "email": "test@example.com", "password": "wrong" }
```

Ожидание: `401`:
```json
{"error": {"code": "invalid_credentials", "message": "Invalid credentials"}}
```

### 11. Дубликат email

```
POST /api/v1/auth/register
Body: { "email": "test@example.com", "password": "anotherpass" }
```

Ожидание: `409`:
```json
{"error": {"code": "email_already_taken", "message": "Email is already registered"}}
```

### 12. Профиль без авторизации

Logout, потом сразу:
```
GET /api/v1/profile
```

Ожидание: `401 unauthorized`.

### 13. CSRF без токена

В Postman временно убрать pre-request скрипт. Любой POST/PATCH/DELETE:

Ожидание: `403 csrf_missing`.

## Что проверяет каждый шаг

| Шаг | Что валидируется |
|---|---|
| 1 | Сессия создаётся, cookie ставится, CSRF выдан |
| 2 | Регистрация апгрейдит anonymous-сессию в customer'а, CSRF ротирован |
| 3 | GetAuthSession подгружает user из PG и заполняет role/user_id |
| 4 | GetProfile требует customer-роль и достаёт User из PG |
| 5 | PatchProfile применяет частичный patch, allergens/dietary массивами |
| 6 | ChangePassword сверяет текущий пароль через bcrypt и меняет |
| 7 | Logout удаляет Redis-сессию |
| 8 | Без cookie/после logout сессия пересоздаётся как anonymous |
| 9 | Login проверяет новый пароль, привязывает сессию к существующему user_id |
| 10 | Wrong password → 401 invalid_credentials |
| 11 | Email unique constraint → 409 email_already_taken |
| 12 | Auth middleware (через requireUser) → 401 |
| 13 | CSRF middleware → 403 |

# Меню (public + admin)

Раздел тестирует фичу `menu`. Перед началом — пройди `Auth` runbook выше (хотя бы шаги 1–2), чтобы в БД появился customer.

## Подготовить admin

В БД нет ручки «создать админа», поэтому делаем это вручную через psql.

1. Зарегистрировать customer'а через коллекцию (`Auth → 1.Bootstrap`, `Auth → 2.Register`) с `email = admin@example.com`, `password = adminpass1`.
2. Промоутить его до admin'а:
   ```sh
   docker compose exec postgres psql -U restaurant -d restaurant -c "UPDATE users SET role='admin' WHERE email='admin@example.com';"
   ```
   `restaurant` — значения `POSTGRES_USER`/`POSTGRES_DB` из `.env`. Хардкод сделан намеренно, чтобы команда работала одинаково в bash, PowerShell и cmd без возни с экранированием `$`.
3. Залогиниться через коллекцию (`Admin — login → Bootstrap session`, `Admin — login → Login as admin`). После этого `is_guest=false`, `role=admin`, новый CSRF подхватывается автоматически.

> Алтернатива без второго юзера: промоуть уже зарегистрированного `test@example.com` — тогда `adminEmail`/`adminPassword` в коллекции замени на его значения.

## Public read

Эти три эндпойнта доступны всем (security: []), без cookie/CSRF.

### 14. Список категорий

```
GET /api/v1/categories
```
Ожидание: `200`, `{"items": [...]}`. Сразу после миграций — пустой массив; после шагов 16+ — будет минимум одна категория.

### 15. Каталог блюд

```
GET /api/v1/menu?limit=20&offset=0
```
Ожидание: `200`, `{"items": [...], "total": N, "limit": 20, "offset": 0}`. По умолчанию `available=true` фильтр на сервере: попадают только блюда `is_available=true`.

С фильтрами:
```
GET /api/v1/menu?category_id={{categoryId}}&exclude_allergens=gluten&exclude_allergens=dairy&dietary=vegan&tag_ids={{tagId}}&q=&limit=20&offset=0
```
Семантика:
- `category_id` — точное совпадение,
- `exclude_allergens` (повторяется в query) — блюдо исключается, если в `allergens` есть **хотя бы один** из перечисленных (PG `&&`),
- `dietary` — блюдо включается, если **все** запрошенные диеты содержатся в `dietary` блюда (PG `@>`),
- `tag_ids` — блюдо включается, если **хотя бы один** из тегов привязан,
- `q` — `ILIKE %name%`,
- `available` (bool) — если задан `false`, сервер вернёт всё, включая стоп-лист.

### 16. Детали блюда

```
GET /api/v1/menu/{{dishId}}
```
Ожидание: `200`, объект `Dish` со всеми полями + массив `tags` (Tag-объекты, не id-шники).

### 17. Несуществующее блюдо

```
GET /api/v1/menu/999999
```
Ожидание: `404`, `{"error":{"code":"dish_not_found", ...}}`.

## Admin CRUD

Все запросы под `/admin/*` требуют сессии admin'а (см. «Подготовить admin» выше) и валидный `X-CSRF-Token`.

### 18. Создать категорию

```
POST /api/v1/admin/categories
{"name": "Супы", "sort_order": 10, "is_available": true}
```
Ожидание: `201`. ID категории сохраняется в `categoryId`.

### 19. Обновить категорию

```
PATCH /api/v1/admin/categories/{{categoryId}}
{"sort_order": 5}
```
Ожидание: `200`, `sort_order=5`.

### 20. Дубликат имени категории → 409

```
POST /api/v1/admin/categories
{"name": "Супы"}
```
Ожидание: `409`, `code=category_name_taken`.

### 21. Создать тег

```
POST /api/v1/admin/tags
{"name": "Острое", "slug": "spicy", "color": "#E53935"}
```
Ожидание: `201`. ID сохраняется в `tagId`.

### 22. Список тегов

```
GET /api/v1/admin/tags
```
Ожидание: `200`, `{"items": [...]}`.

### 23. Обновить тег

```
PATCH /api/v1/admin/tags/{{tagId}}
{"color": "#FF0000"}
```
Ожидание: `200`.

### 24. Создать блюдо

```
POST /api/v1/admin/menu
{
  "name": "Борщ с говядиной",
  "description": "Классический борщ",
  "composition": "Говядина, свёкла, капуста, картофель, лук, сметана",
  "price_minor": 45000,
  "currency": "RUB",
  "calories_kcal": 320, "protein_g": 18.5, "fat_g": 15.0, "carbs_g": 22.0,
  "portion_weight_g": 350,
  "cuisine": "russian",
  "category_id": {{categoryId}},
  "allergens": ["dairy"],
  "dietary": [],
  "tag_ids": [{{tagId}}],
  "is_available": true
}
```
Ожидание: `201`, в ответе `tags: [...]` уже подгружены. ID сохраняется в `dishId`.

### 25. Обновить блюдо

```
PATCH /api/v1/admin/menu/{{dishId}}
{"price_minor": 49000, "description": "Обновлённое описание"}
```
Ожидание: `200`, поля обновлены. Если в теле есть `tag_ids` — связи перепривязываются полностью.

### 26. Загрузить картинку

```
POST /api/v1/admin/menu/{{dishId}}/image
Content-Type: multipart/form-data
field: file = <photo.jpg>
```
В Postman: вкладка `Body → form-data → key=file, type=File`, выбрать файл `.jpg`/`.png`/`.webp` ≤ 5 MiB.

Ожидание: `200`, `Dish` с `image_url=http://localhost:9000/restaurant/dishes/<id>-<ts>.<ext>`. Этот URL открывается в браузере (anonymous-read bucket настроен в `minio-init`).

Ошибки:
- слишком большой файл → `413 image_too_large`,
- `Content-Type` не из (jpeg/png/webp) → `415 image_unsupported_type`,
- запрос без поля `file` → `400`.

### 27. Снять блюдо со стоп-листа (soft delete)

```
DELETE /api/v1/admin/menu/{{dishId}}
```
Ожидание: `204`. В БД блюдо физически остаётся, просто `is_available=false`. После этого `GET /menu` его не вернёт (по умолчанию фильтр `is_available=true`).

### 28. Удалить тег

```
DELETE /api/v1/admin/tags/{{tagId}}
```
Ожидание: `204`. Связи в `dish_tags` удаляются каскадом.

### 29. Удалить категорию с блюдами → 409

```
DELETE /api/v1/admin/categories/{{categoryId}}
```
Ожидание: `409`, `code=category_has_dishes`. Soft-delete блюда не освобождает категорию — нужно либо физически удалить блюдо в БД, либо переместить его в другую категорию через `PATCH /admin/menu/{id}`.

### 30. Customer не может вызвать admin endpoint

Залогинься под обычным `test@example.com` и:
```
POST /api/v1/admin/categories
{"name": "Закуски"}
```
Ожидание: `403`.

## Что проверяет каждый шаг (меню)

| Шаг | Что валидируется |
|---|---|
| 14 | Public list categories |
| 15 | Public list dishes + GIN-фильтры (allergens/dietary/tags) + ILIKE-поиск |
| 16 | Public get dish + подтянутые tags |
| 17 | 404 от sentinel ErrDishNotFound |
| 18 | Create category, unique constraint name |
| 19 | Patch category, partial update |
| 20 | Unique violation на name → 409 category_name_taken |
| 21 | Create tag |
| 22 | List tags для admin-формы |
| 23 | Patch tag |
| 24 | Create dish — включая tags M2M в одной транзакции |
| 25 | Patch dish, перепривязка тегов через UpdateDish(tagIDs!=nil) |
| 26 | Multipart upload → S3 → image_url, mime-валидация, лимит размера |
| 27 | Soft-delete через SetDishAvailability(false) |
| 28 | Delete tag, каскад dish_tags |
| 29 | DeleteCategory блокируется при наличии блюд |
| 30 | requireAdmin() в delivery → 403 для customer |

# RAG (Qdrant)

Векторное хранилище для семантического поиска по меню. Используется в шаге A3
(Cohere embed запроса → Qdrant search с pre-filter → Cohere rerank → LLM).
Сейчас (A2) только подготовка: индексируем все блюда, проверяем коллекцию.

## Подготовка

1. `make docker-up` — поднимает qdrant на `:6333` (HTTP) и `:6334` (gRPC).
2. В `.env` должен быть `COHERE_API_KEY` (получить — https://dashboard.cohere.com).
3. После накатки меню (`make seed`) запустить индексацию:

```sh
make embed-menu
```

Идемпотентно: повторный запуск перезапишет векторы. Создаёт коллекцию и
payload-индексы при первом запуске.

В логе ожидается:
```
INFO dishes loaded count=193
INFO indexed batch from=0 to=96 size=96
INFO indexed batch from=96 to=192 size=96
INFO indexed batch from=192 to=193 size=1
INFO embed-menu done dishes=193 qdrant_points=193 collection=dishes
```

## Веб-UI Qdrant

http://localhost:6333/dashboard — браузер для коллекций.

В верхнем правом углу — поле «API Key». Подставить значение `QDRANT_API_KEY` из `.env`.

После авторизации:
- слева вкладка **Collections → dishes** — конфиг коллекции (size: 1024, distance: Cosine);
- вкладка **Points** — список точек с payload, можно листать;
- вкладка **Visualize** — UMAP-проекция векторов (видно кластеры по «острому» / «десерты» / т.д.).

## Debug через REST API

Для всех запросов нужен заголовок `api-key: <QDRANT_API_KEY>`.

### Список коллекций
```sh
curl -s http://localhost:6333/collections \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)"
```

### Конфиг коллекции
```sh
curl -s http://localhost:6333/collections/dishes \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)" | jq .
```

### Количество точек
```sh
curl -s http://localhost:6333/collections/dishes/points/count \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)" \
  -H "Content-Type: application/json" \
  -d '{"exact":true}' | jq .
```
Ожидание: `result.count = 193`.

### Прочитать точку по dish_id
```sh
curl -s http://localhost:6333/collections/dishes/points/32 \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)" | jq .result.payload
```
Ожидание: payload с полями `dish_id`, `category_id`, `cuisine`, `allergens`, `dietary`, `tag_ids`, `price_minor`, `is_available`, опц. `calories_kcal`, `portion_weight_g`. **НЕ должно быть** `name`/`description`/`composition` — они живут в PG.

### Найти точки по фильтру (без vector search)
```sh
# Все блюда с аллергеном "dairy"
curl -s http://localhost:6333/collections/dishes/points/scroll \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)" \
  -H "Content-Type: application/json" \
  -d '{
    "limit": 5,
    "filter": { "must": [ { "key": "allergens", "match": { "value": "dairy" } } ] },
    "with_payload": ["dish_id", "cuisine", "allergens"]
  }' | jq .
```

### Удалить коллекцию (если хочется переиндексировать с нуля)
```sh
curl -s -X DELETE http://localhost:6333/collections/dishes \
  -H "api-key: $(grep QDRANT_API_KEY .env | cut -d= -f2)" | jq .
make embed-menu
```

## Полная перезаливка меню

Когда меняешь `seed/menu.json` целиком (или хочешь перезагрузить из чистого состояния), **простой `make seed` не поможет**: в `cmd/seed` блюда вставляются insert-only — если блюдо с таким `name` уже есть в PG, оно не обновится. Картинки в MinIO и точки в Qdrant тоже останутся старыми (по `image_key` / `dish_id`).

Для полной перезаливки есть два таргета:

```sh
make wipe-menu     # дроп: PG menu-таблицы, MinIO dishes/*, Qdrant collection
make reset-menu    # wipe-menu + seed + embed-menu (всё за раз)
```

Что делает `wipe-menu`:

| Хранилище | Действие |
|---|---|
| PG | `TRUNCATE dish_tags, dishes, tags, categories RESTART IDENTITY CASCADE` (id-шники сбрасываются на 1; users/chats/sessions **не трогаются**) |
| MinIO | `mc rm -r local/$MINIO_BUCKET/dishes` (картинки блюд; admin-аватарки/прочее не затрагивается) |
| Qdrant | `DELETE /collections/dishes` |

Идемпотентно: если что-то уже отсутствует, таргет не падает.

**Когда использовать:**

- сменился `seed/menu.json` (новые блюда, изменённые описания/цены/состав),
- хочешь начать «с чистого» состояния перед демо,
- ловишь рассинхрон между PG и Qdrant (id блюда в Qdrant нет, в PG есть — или наоборот).

**Не использовать**, если нужно поменять только description у уже залитых блюд — для этого есть `make seed-descriptions`. Он работает по `name`, ничего не дропает.

После `make reset-menu`:
- `categories.id`, `tags.id`, `dishes.id` начинаются с 1 (это важно, если у тебя есть какие-то фронт-id-шники в коллекционных переменных Postman — обновить);
- активные сессии customer'ов не сбрасываются (они в Redis), но их `chat_messages.recommended_dish_ids`/`meta.reranked_ids` могут указывать на несуществующие id (старые чаты лучше удалить руками).

## Что проверяет `make embed-menu`

| Проверка | Как |
|---|---|
| Коллекция создаётся с правильной размерностью | UI: `dishes.config.params.vectors.size == 1024` |
| Создались payload-индексы | UI: `dishes.config.params.payload_schema` — должны быть allergens/dietary/cuisine/category_id/tag_ids/price_minor/is_available/calories_kcal/portion_weight_g |
| Залились все блюда | `points/count` == количеству блюд в PG |
| Payload корректен (без текстовых полей) | curl одной точки: `{dish_id, category_id, cuisine, allergens, dietary, tag_ids, price_minor, is_available, ...}` |
| Идемпотентность | повторный `make embed-menu` не падает, count тот же |

# Chats

Чат с ассистентом (на A1 — эхо-заглушка). Перед прогоном пройди шаги 1–2 из секции Auth, чтобы появился customer.

## 31. Активный чат (auto-create)

```
GET /api/v1/chats/active
```
Возвращает текущий чат пользователя. Если у него нет чата или последний устарел (по умолчанию — `chat.usecase.auto_new_chat_after = 6h`), создаётся новый и возвращается. ID активного чата сохраняется в коллекционную переменную `chatId`.

Ожидание: `200`, `Chat`:
```json
{
  "id": "<uuid>",
  "user_id": "<uuid>",
  "title": null,
  "created_at": "...",
  "last_message_at": "..."
}
```

## 32. Список чатов

```
GET /api/v1/chats?limit=20&offset=0
```
Ожидание: `200`, `{"items": [...], "total": N, "limit": 20, "offset": 0}`. Сортировка — DESC по `last_message_at`.

## 33. Создать новый чат явно

```
POST /api/v1/chats
{ "title": "Обед в офисе" }
```
Ожидание: `201`, `Chat`. `title` опционален.

## 34. Отправить сообщение (SSE-стрим)

```
POST /api/v1/chats/{{chatId}}/messages
Headers: X-CSRF-Token (auto), Content-Type: application/json
Body: { "content": "Что у вас острого без молочки?" }
```

Ожидание: `200`, `Content-Type: text/event-stream`, тело — три события:
```
event: meta
data: {"message_id":"<uuid>","recommended_dish_ids":[]}

event: token
data: {"delta":"echo: Что у вас острого без молочки?"}

event: done
data: {"tokens_in":0,"tokens_out":0,"latency_ms":<N>}
```

На A1 ассистент отвечает эхо-заглушкой одним токеном, реальный LLM появится в шаге A3. И user-сообщение, и assistant-сообщение пишутся в `chat_messages` атомарно (вместе с `last_message_at` чата).

## 35. История сообщений

```
GET /api/v1/chats/{{chatId}}?messages_limit=50
```
Ожидание: `200`, `ChatWithMessages`:
- `chat` — мета чата,
- `messages` — массив, отсортированный **ASC по created_at** (старые → новые),
- `has_more` — есть ли ещё более старые сообщения за пределами лимита.

Курсорная пагинация:
```
GET /api/v1/chats/{{chatId}}?messages_before={{messageId}}&messages_limit=50
```
Вернёт N сообщений строго старше указанного `messages_before`.

## 36. Пустое сообщение → 400

```
POST /api/v1/chats/{{chatId}}/messages
Body: { "content": "" }
```
Ожидание: `400 validation_failed`. Также 400 если content состоит только из пробелов (валидация на уровне usecase, чтобы не писать в БД мусор).

## 37. Несуществующий чат → 404

```
GET /api/v1/chats/00000000-0000-0000-0000-000000000000
```
Ожидание: `404 chat_not_found`.

## 38. Чужой чат → 403

Если зарегистрированный customer попытается обратиться к `chatId` другого пользователя:
```
GET /api/v1/chats/{{chatIdДругогоЮзера}}
```
Ожидание: `403 access_denied` (`Chat does not belong to this user`).

## 39. Чат без авторизации → 401

```
POST /api/v1/auth/logout    (сначала вышли)
GET /api/v1/chats/active
```
Ожидание: `401 unauthorized`. На A1 чат требует `session.UserID != nil` — гость без явного `Register/Login` его не видит. Lazy-создание guest-юзера для чатов будет добавлено отдельной задачей.

## 40. Удалить чат

```
DELETE /api/v1/chats/{{chatId}}
```
Ожидание: `204`. Сообщения чата удаляются каскадом (FK `chat_messages.chat_id ON DELETE CASCADE`).

## Что проверяет каждый шаг (chats)

| Шаг | Что валидируется |
|---|---|
| 31 | GetActive: auto-create нового / возврат свежего; client сохраняет id |
| 32 | List по убыванию last_message_at, total/limit/offset |
| 33 | Принудительное создание нового чата |
| 34 | SendMessage: атомарная запись user+assistant в БД, SSE-формат meta/token/done |
| 35 | GetWithMessages: ASC-сортировка для UI, has_more, курсор before |
| 36 | Валидация пустого/whitespace-only content |
| 37 | Sentinel ErrChatNotFound → 404 |
| 38 | Проверка владельца чата → 403 |
| 39 | Требуется session.UserID (на A1 — без lazy-guest) |
| 40 | Cascade delete сообщений |

# RAG + LLM (A3)

С шага A3 ассистент отвечает реальным текстом, не эхо. Pipeline:

```
embed (Cohere search_query)
  → Qdrant.search с pre-filter (is_available=true, must_not allergens, must dietary)
  → Cohere.rerank → top-5
  → menu.GetDishesByIDs из PG
  → OpenRouter chat-completions (LLM stream)
  → SSE: meta(recommended_dish_ids) → token* → done(usage, model, latency)
```

## Что нужно перед запуском

1. **Ключи в `.env`** — без них приложение не стартует:
   ```
   COHERE_API_KEY=<твой ключ>
   OPENROUTER_API_KEY=<твой ключ>
   QDRANT_API_KEY=<любая строка>
   ```
2. **Стек поднят**: `make docker-up` (поднимет qdrant в т.ч.).
3. **Меню в PG**: `make seed`.
4. **Меню в Qdrant**: `make embed-menu`. После — в Qdrant UI (http://localhost:6333/dashboard) на вкладке `dishes` должно быть 193 точки.

## 41. Чат с реальным LLM

Последовательность в Postman:
1. Auth → шаг 1 (Bootstrap session) → шаг 2 (Register) или Login.
2. Chats → шаг 1 (Active chat). В коллекционной переменной `chatId` — id активного чата.
3. Чтобы pre-filter мог что-то фильтровать — заполни профиль (это не обязательно для базового теста, но позволит увидеть hard-фильтры в действии):
   ```
   PATCH /api/v1/profile
   { "allergens": ["dairy"], "dietary": [] }
   ```
4. Chats → шаг 4 (Send message) с осмысленным `content`, например:
   ```json
   { "content": "что у вас острого без молочки?" }
   ```

В Postman вкладка **Body** покажет SSE-ленту:
```
17:55:10.716  meta   {"message_id":"...","recommended_dish_ids":[6,32,76,40,79]}
17:55:11.121  token  {"delta":"Реко"}
17:55:11.183  token  {"delta":"мендую"}
...
17:55:12.890  token  {"delta":"."}
17:55:13.012  done   {"latency_ms":2300,"tokens_in":1247,"tokens_out":156,"model":"meta-llama/llama-3.3-70b-instruct:free"}
```

После закрытия соединения:
- В БД появится новый `chat_messages` с `role='assistant'`, `content` — чистый текст ответа (JSON-блок с `recommended_dish_ids` уже вырезан), `recommended_dish_ids = массив id`, `meta` — JSONB с полной телеметрией.
- В Postman → шаг 5 (Get chat with messages) — увидишь и user-, и assistant-сообщение с `recommended_dish_ids`.

## 42. Проверка hard-фильтра аллергенов

Цель — убедиться, что Qdrant pre-filter режет по `allergens`, и блюда с `dairy` физически не попадают в выдачу при запросе типа «сырную пиццу».

1. В профиле выставь `"allergens": ["dairy"]`.
2. Send message: `{"content":"хочу пиццу с сыром"}`.

Ожидание: ассистент честно говорит «такого без молочки нет» (или предложит безлактозную альтернативу), потому что **пицца сырная** в Qdrant отфильтрована pre-filter'ом и в context LLM она не приходит. Если бы аллергены проверялись только промптом, можно было бы получить рекомендацию пиццы — у нас этого не произойдёт.

В `chat_messages.meta.retrieved_ids` для этого сообщения **не должно** быть `dish_id` блюд с `dairy` в составе.

## 43. Контекст диалога

1. Send message: `{"content":"что у вас острого?"}`. Ассистент рекомендует, например, Том-ям.
2. Send message: `{"content":"расскажи про Том-ям подробнее"}`.

Ожидание: ассистент описывает именно Том-ям, не предлагает кучу новых блюд. Это работает за счёт:
- `contextualQuery` = последние 2 user-сообщения + текущий → лучший retrieval;
- last 6 messages истории передаются в LLM как `messages: [system, user, assistant, user, ...]`;
- `prior_recommended` (id из прошлых assistant-ответов) идёт в prompt как «ранее рекомендовано».

В `meta.reranked_ids` второго сообщения должен быть `dish_id` Том-яма.

## 44. Off-topic

Send message: `{"content":"расскажи как решить квадратное уравнение"}`.

Ожидание: короткий вежливый отказ типа «я могу помочь только с выбором блюд». Это поведение задано в system prompt'е (правило 5).

В `meta.reranked_ids` может быть пусто или содержать рандомные блюда (Qdrant что-то вернёт по semantic-сходству), но LLM по инструкции **не рекомендует** их и в `recommended_dish_ids` будет `[]`.

## 45. Прощание

Send message: `{"content":"спасибо, понятно"}`.

Ожидание: короткий ответ «приятного аппетита» / «обращайтесь» без рекомендаций. `recommended_dish_ids` — пустой массив.

## 46. SSE через `curl`

В Postman SSE парсится в виде ленты, но если хочешь увидеть raw-формат:

```sh
# Сначала залогинься, чтобы появились cookies.txt
curl -i -c cookies.txt -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"newsecret456"}'

# Активный чат
CHAT=$(curl -s -b cookies.txt http://localhost:8081/api/v1/chats/active | jq -r .id)

# CSRF из cookie
CSRF=$(awk '/csrf/{print $7}' cookies.txt)

# Отправить с -N (no-buffering), чтобы видеть стрим
curl -N -b cookies.txt \
  -H "X-CSRF-Token: $CSRF" \
  -H 'Content-Type: application/json' \
  -X POST "http://localhost:8081/api/v1/chats/$CHAT/messages" \
  -d '{"content":"что у вас острого без молочки?"}'
```

## Дебаг через логи и БД

**Логи приложения** (включая ошибки Cohere/Qdrant/OpenRouter):
```sh
docker compose logs -f app | grep -i "cohere\|qdrant\|openrouter\|send message"
```

**Полная телеметрия в `meta`** — JSONB по последнему ответу:
```sh
docker compose exec postgres psql -U restaurant -d restaurant -c \
  "SELECT id, LEFT(content, 60) AS preview, recommended_dish_ids, meta
   FROM chat_messages
   WHERE role='assistant'
   ORDER BY created_at DESC
   LIMIT 3;"
```

В `meta` увидишь:
- `latency_ms` — время от записи user-msg до записи assistant-msg
- `tokens_in` / `tokens_out` — учёт OpenRouter
- `finish_reason` — `stop` (нормально) / `length` (упёрлось в `max_tokens`) / `content_filter`
- `model` — фактическая модель, которой ответил OpenRouter (если включён фолбек, может отличаться от запрошенной)
- `retrieved_ids` — top-K (~20) от Qdrant ДО рерэнкинга
- `reranked_ids` — top-N (~5) ПОСЛЕ рерэнкинга, именно эти блюда были в промпте LLM
- `llm_recommended` — id блюд, которые сама LLM явно упомянула в JSON-блоке в финале ответа

## Что проверяет каждый шаг (RAG + LLM)

| Шаг | Что валидируется |
|---|---|
| 41 | Полный pipeline: embed → search → rerank → LLM stream |
| 42 | Hard-фильтр аллергенов через Qdrant pre-filter (must_not) |
| 43 | Учёт истории диалога: anchor (первый user-msg) + последние HistoryRecentPairs пар в prompt |
| 44 | Off-topic не приводит к рекомендациям (поведение из system prompt) |
| 45 | Прощание не триггерит RAG-выдачу |
| 46 | Raw SSE через curl: формат meta/token/done корректный |
| 9.1 | Companion retrieval: к мясному ужину поднимаются соус/гарнир/хлеб/десерт/напиток (см. `meta.companion_ids` в БД) |
| 9.2 | Anchor: после уточнения ассистент остаётся в рамке изначального запроса |

## Companion retrieval (как это устроено)

В `runRAG` после основного `embed → search → rerank → top-N` запускается дополнительный проход:

1. Грузим список категорий из PG (там уже был, теперь используется ещё и для маппинга «имя → id»).
2. Для каждой companion-категории из конфига `rag.chat.companions` (по умолчанию: Соусы, Гарниры, Закуски, Десерты, Напитки безалкогольные) делаем `Qdrant.Search`:
   - тот же эмбеддинг текущего сообщения,
   - тот же базовый pre-filter (allergens/dietary/is_available),
   - **дополнительно** `must: { key: "category_id", match: <id> }`,
   - top_k=5, без rerank, берём top-1.
3. Дедуплицируем с уже выбранными в primary-выдаче.
4. Загружаем подобранные companion-блюда из PG одной транзакцией.
5. В prompt LLM добавляется отдельный блок **«=== Сопровождение (на выбор, если уместно к запросу) ===»**.

Алкоголь в companion НЕ включён — система-промпт явно запрещает предлагать его без явной просьбы гостя.

## Anchor + history pairs (как это устроено)

В `loadHistory`:

1. **Anchor**: новый запрос `FindFirstUserMessage(chatID)` — единственный SELECT с ORDER BY created_at ASC LIMIT 1. Возвращает хронологически первое user-сообщение чата (исключая current).
2. **Recent pairs**: последние `HistoryRecentPairs * 2` сообщений (по умолчанию 4 = 2 пары). 
3. **Дедуп**: если anchor совпадает с первым в recent (короткий чат) — anchor сбрасывается.
4. В `messages` LLM итог: `[system, anchor?, ...recent (chronological), current_with_RAG_context]`.

Эффект: даже после 10+ ходов ассистент видит изначальный запрос гостя в начале сообщений и не теряет рамку.

# Filters & profiles (теги, аллергены, диеты, hard-фильтры RAG)

Эта секция собирает в одно место способы проверить, что **фильтры каталога**
(`/menu?...`) и **hard-фильтры профиля в чате** (Qdrant pre-filter по
`allergens` / `dietary`) работают как договорено. Соответствует папке
**Filters & profiles** в Postman-коллекции.

## Что в seed-меню

Это нужно держать в голове, когда смотришь на total и items:

- **Аллергены**: `gluten` (41 блюдо), `dairy` (52), `soy` (12), `nuts` (11),
  `shellfish` (10), `fish` (8), `eggs` (8), `sesame` (7), `mustard` (7),
  `celery` (3), `peanuts` (2).
- **Диеты**: `vegetarian` (113), `vegan` (87), `lactose_free` (141),
  `gluten_free` (152), `halal` (10).
- **Кухни**: `european` (122), `american` (31), `italian` (22), `asian` (9),
  `japanese` (4), `french` (4), `russian` (1).
- **Tag slugs**: `hit`, `new`, `spicy`, `chef`, `big`, `share`, `light`,
  `coffee`, `tea`, `lemonade`, `wine`, `beer`.

## 47. Каталог: исключение одного аллергена

```
GET /api/v1/menu?exclude_allergens=dairy&limit=5
```
Ожидание: `200`, `total` строго меньше, чем без фильтра. Ни в одном `items[].allergens` нет `dairy`.

## 48. Каталог: исключение нескольких аллергенов (OR)

```
GET /api/v1/menu?exclude_allergens=dairy&exclude_allergens=gluten&limit=5
```
Семантика: блюдо исключается, если **хотя бы один** из перечисленных аллергенов в составе (PG `allergens && excluded`). `total` ещё ниже.

## 49. Каталог: одна диета

```
GET /api/v1/menu?dietary=vegan&limit=10
```
Ожидание: `total ≈ 87`, у всех блюд в `dietary` есть `vegan`.

## 50. Каталог: пересечение диет (AND)

```
GET /api/v1/menu?dietary=vegan&dietary=gluten_free
```
Семантика: блюдо включается, если **все** запрошенные диеты есть в `dietary` (PG `@>`). Это пересечение двух множеств.

## 51. Каталог: халяль (edge — маленькая выдача)

```
GET /api/v1/menu?dietary=halal
```
Ожидание: `total = 10`. Удобно для проверки, что фильтр не падает на маленьких множествах.

## 52. Каталог: поиск по q

```
GET /api/v1/menu?q=стейк&limit=20
GET /api/v1/menu?q=том-ям
```
Ожидание: `ILIKE %q%` по `name`. Для `том-ям` должно прийти 1–2 блюда.

## 53. Каталог: tag_ids

```
# 1) GET /admin/tags  (под admin'ом) → находим id тега 'spicy'
# 2) GET /api/v1/menu?tag_ids=<id>&limit=25
```
Ожидание: у всех блюд в `tags[].slug` есть `spicy`. В seed таких ~20.

В Postman-коллекции это автоматизировано: шаг 15 берёт id, шаг 16 фильтрует.

## 54. Чат: hard-фильтр по аллергену в профиле

```
PATCH /profile  { "allergens": ["dairy"], "dietary": [] }
POST  /chats/{id}/messages  { "content": "хочу пиццу с сыром" }
```
Ожидание: ассистент честно отвечает «без молочки нет такого» или предлагает безлактозную альтернативу. В `chat_messages.meta.reranked_ids` **не должно быть** блюд с `dairy` в составе — Qdrant pre-filter их физически отрезал (`must_not allergens=dairy`).

## 55. Чат: hard-фильтр по диете в профиле

```
PATCH /profile  { "allergens": [], "dietary": ["vegan"] }
POST  /chats/{id}/messages  { "content": "что взять на ужин?" }
```
Ожидание: рекомендации только из веганских блюд. Проверить можно так:

```sh
docker compose exec postgres psql -U restaurant -d restaurant -c \
  "SELECT id, name, dietary FROM dishes WHERE id = ANY(
     (SELECT (meta->'reranked_ids')::jsonb FROM chat_messages
       WHERE role='assistant' ORDER BY created_at DESC LIMIT 1)::int[]
   );"
```
В каждой строке поле `dietary` должно содержать `vegan`.

## 56. Чат: edge — все запрошенное отрезано фильтром

```
PATCH /profile  { "allergens": ["shellfish","fish"], "dietary": [] }
POST  /chats/{id}/messages  { "content": "что-нибудь морское" }
```
Ожидание: ассистент не выдумывает, а честно говорит, что подходящего нет, и предлагает мясное / куриное / овощное альтернативное (правило 2 из system prompt).

## 57. Cleanup профиля

```
PATCH /profile  { "allergens": [], "dietary": [] }
```
Чтобы профиль не мешал другим тестам. В Postman-коллекции это шаг 17 в папке Filters & profiles.

## Что валидирует каждый шаг (filters & profiles)

| Шаг | Что валидируется |
|---|---|
| 47 | Один `exclude_allergens` режет правильное подмножество |
| 48 | Несколько `exclude_allergens` работают как OR (PG `&&`) |
| 49 | Один `dietary` фильтр |
| 50 | Несколько `dietary` работают как AND (PG `@>`) |
| 51 | Маленькое множество (halal) — на нём проще проверить вручную |
| 52 | `q` (ILIKE по name) |
| 53 | `tag_ids` (HAS-ANY) |
| 54 | Qdrant pre-filter по аллергенам режет до LLM |
| 55 | Qdrant pre-filter по диетам |
| 56 | Поведение «нет подходящего» — LLM не выдумывает |
| 57 | Cleanup |

## Если что-то не работает

| Симптом | Проверь |
|---|---|
| Сразу `event: error` после `meta` | Логи приложения. Скорее всего Cohere или OpenRouter rate-limit / неверный ключ. |
| LLM отвечает «не нашёл подходящих блюд» на простой запрос | `meta.retrieved_ids` в БД пустой → проверь что `make embed-menu` отработал и в Qdrant 193 точки. |
| LLM рекомендует блюдо с аллергеном | `meta.reranked_ids` — есть ли там запрещённый id? Если да — проверь, что `allergens` в БД-блюде корректные (`SELECT allergens FROM dishes WHERE id=...`) и pre-filter в Qdrant payload-индексе настроен. |
| Стрим приходит с большой задержкой (5+ сек до первого token) | OpenRouter free-tier бывает медленный, либо `first_token_timeout` в yaml нужно увеличить. |
| `done` приходит с `tokens_in=0, tokens_out=0` | OpenRouter не вернул блок `usage` в стриме (бывает на free-tier). Не страшно — телеметрия неполная, ответ нормальный. |

## Если что-то не так

Посмотреть чаты пользователя в PG:
```sh
docker compose exec postgres psql -U restaurant -d restaurant -c "SELECT id, user_id, title, last_message_at FROM chats ORDER BY last_message_at DESC LIMIT 10;"
docker compose exec postgres psql -U restaurant -d restaurant -c "SELECT chat_id, role, LEFT(content, 60) AS preview, created_at FROM chat_messages ORDER BY created_at DESC LIMIT 20;"
```

Сбросить чаты конкретного пользователя:
```sh
docker compose exec postgres psql -U restaurant -d restaurant -c "DELETE FROM chats WHERE user_id IN (SELECT id FROM users WHERE email='test@example.com');"
```

## Если что-то не так

Логи приложения:
```sh
docker compose logs -f app
```

Посмотреть содержимое Redis-сессии:
```sh
docker compose exec redis redis-cli -a $REDIS_PASSWORD KEYS 'session:*'
docker compose exec redis redis-cli -a $REDIS_PASSWORD GET session:<uuid>
```

Посмотреть user в PG:
```sh
docker compose exec postgres psql -U restaurant -d restaurant -c "SELECT id, email, role FROM users;"
```

Посмотреть меню в PG:
```sh
docker compose exec postgres psql -U restaurant -d restaurant -c "SELECT id, name, category_id, is_available FROM dishes;"
docker compose exec postgres psql -U restaurant -d restaurant -c "SELECT * FROM categories; SELECT * FROM tags; SELECT * FROM dish_tags;"
```

Посмотреть объекты в MinIO (логин/пароль из `.env::MINIO_ROOT_*`):
```sh
docker compose exec minio-init mc alias set local http://minio:9000 restaurant Rk19Vc83Dn46QtMyHs07
docker compose exec minio-init mc ls -r local/restaurant
```
Веб-консоль: http://localhost:9001.
Веб-консоль: http://localhost:9001 (логин/пароль из `.env`).

Сбросить всё:
```sh
make docker-down
docker volume rm ai-restaurant-assistant-backend_postgres_data ai-restaurant-assistant-backend_redis_data
make docker-up
```
