# Ручное тестирование auth-флоу

Минимальный runbook для прогона `auth + profile` end-to-end через Postman / curl.

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
