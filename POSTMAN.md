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
docker compose exec postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT id, email, role FROM users;"
```

Сбросить всё:
```sh
make docker-down
docker volume rm ai-restaurant-assistant-backend_postgres_data ai-restaurant-assistant-backend_redis_data
make docker-up
```
