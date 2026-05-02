# ai-restaurant-assistant-backend

REST API бэкенд цифрового ассистента рекомендаций блюд для посетителей ресторана.

## Стек

- **Go 1.24** — основное приложение
- **PostgreSQL 16** — основная БД
- **Redis 7** — сессии и кэш
- **Qdrant** — векторная БД для RAG
- **MinIO** — объектное хранилище (фото блюд)
- **OpenRouter** (LLM) + **Cohere** (rerank, опционально)
- **OpenAPI 3** + oapi-codegen

---

## Требования

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (с поддержкой `docker compose`)
- GNU Make
- Go 1.24+ (только для локальной разработки без Docker)
- Python 3 (только для `make group-imports`)

---

## Быстрый старт

```sh
# 1. Скопируй конфиг и заполни секреты
cp .env.example .env

# 2. Заполни в .env обязательные ключи:
#    OPENROUTER_API_KEY=...   (или NVIDIA_API_KEY при LLM_PROVIDER=nvidia)
#    COHERE_API_KEY=...       (если используется rerank)

# 3. Подними весь стек
make docker-up
```

Docker Compose автоматически:
- запустит PostgreSQL, Redis, Qdrant, MinIO;
- применит все миграции через сервис `migrate`;
- создаст bucket в MinIO через `minio-init`;
- соберёт и запустит приложение.

**API доступен по адресу:** `http://localhost:8081/api/v1/`

> Порт задаётся переменной `HTTP_PORT` в `.env` (по умолчанию `8081`).

---

## Сервисы и порты

| Сервис | Порт на хосте | Переменная | Назначение |
|---|---|---|---|
| REST API | `8081` | `HTTP_PORT` | Основное приложение |
| PostgreSQL | `5432` | `POSTGRES_PORT` | Основная БД |
| Qdrant HTTP | `6333` | `QDRANT_HTTP_PORT` | Векторная БД |
| Qdrant gRPC | `6334` | `QDRANT_GRPC_PORT` | Векторная БД (gRPC) |
| MinIO API | `9000` | `MINIO_API_PORT` | S3-совместимое хранилище |
| MinIO Console | `9001` | `MINIO_CONSOLE_PORT` | Веб-интерфейс MinIO |
| Redis | внутренний | — | Сессии / кэш (без публичного порта) |

---

## Веб-интерфейсы и подключение к сервисам

### MinIO Console

Браузерный UI для управления объектным хранилищем (бакеты, файлы, политики).

**URL:** `http://localhost:9001`

Логин и пароль берутся из `.env`:
```
MINIO_ROOT_USER=restaurant
MINIO_ROOT_PASSWORD=<значение из .env>
```

После входа открой бакет `restaurant` → папка `dishes/` содержит фотографии блюд.

---

### Qdrant Dashboard

Браузерный UI для просмотра коллекций, точек и выполнения тестовых поисков.

**URL:** `http://localhost:6333/dashboard`

Для авторизации потребуется API-ключ из `.env`:
```
QDRANT_API_KEY=<значение из .env>
```

В дашборде можно проверить коллекцию `menu` — в ней хранятся векторные представления блюд.

---

### PostgreSQL через pgAdmin

pgAdmin не входит в `docker-compose.yml` — используй локально установленный pgAdmin или любой другой SQL-клиент (DBeaver, TablePlus и т.д.).

**Параметры подключения:**

| Параметр | Значение |
|---|---|
| Host | `localhost` |
| Port | `5432` |
| Database | значение `POSTGRES_DB` из `.env` (по умолчанию `restaurant`) |
| Username | значение `POSTGRES_USER` из `.env` |
| Password | значение `POSTGRES_PASSWORD` из `.env` |
| SSL mode | `disable` |

В pgAdmin: `Servers → Register Server → Connection`, заполни поля выше.

---

## Загрузка данных

После старта стека база пустая. Для загрузки меню и изображений:

```sh
# Загрузить блюда в PostgreSQL и фото в MinIO
make seed

# Проиндексировать блюда в Qdrant (требует COHERE_API_KEY)
make embed-menu

# Полный сброс и перезагрузка всех данных
make reset-menu
```

> `make seed` и `make embed-menu` запускаются на хосте и работают с уже поднятым стеком.

---

## Полезные команды

```sh
make help                          # список всех целей с описанием

# Сборка и тесты
make build                         # собрать бинарь bin/app
make test                          # unit-тесты (с -race)
make test-integration              # интеграционные тесты
make lint                          # golangci-lint
make ci-check                      # fmt + lint + test (как в CI)

# Миграции
make migrate-up                    # применить все миграции
make migrate-down                  # откатить последнюю
make migrate-create NAME=add_table # создать новую миграцию

# OpenAPI
make generate-api                  # сгенерировать код из api/openapi.yaml

# Docker
make docker-up                     # поднять весь стек
make docker-down                   # остановить стек
make docker-logs                   # стримить логи app-сервиса
```

---

## Конфигурация

Все параметры задаются через `.env`. Шаблон — `.env.example`.

Обязательные секреты для полноценной работы (AI-функциональность):

| Переменная | Описание |
|---|---|
| `OPENROUTER_API_KEY` | Ключ OpenRouter для LLM-запросов |
| `COHERE_API_KEY` | Ключ Cohere для rerank и embedding |
| `NVIDIA_API_KEY` | Альтернатива OpenRouter (`LLM_PROVIDER=nvidia`) |

---

## Структура проекта

```
cmd/app/          — точка входа приложения
cmd/seed/         — утилита загрузки меню
cmd/embed-menu/   — утилита индексации в Qdrant
internal/         — бизнес-логика (domain, usecase, adapter)
api/openapi.yaml  — OpenAPI-спецификация (source of truth для HTTP-контракта)
migrations/       — SQL-миграции
configs/          — конфиги приложения
postman/          — Postman-коллекция
```

Архитектурные решения описаны в `docs/ARCHITECTURE.md`.
