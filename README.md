# ai-restaurant-assistant-backend

REST API бэкенд цифрового ассистента рекомендаций блюд.

## Стек

- Go 1.24
- PostgreSQL 16, Redis 7, Qdrant
- OpenRouter (LLM, embeddings), Cohere (rerank, опц.)
- OpenAPI 3 + oapi-codegen

## Локальный запуск

```sh
cp .env.example .env
# заполни секреты
make docker-up
```

Поднимаются `postgres`, `redis`, `qdrant`, миграции применяются один раз через
сервис `migrate`, потом стартует `app`. API доступен на `http://localhost:8081/api/v1/`.

## Полезные команды

```sh
make help              # список целей
make build             # собрать бинарь
make test              # unit-тесты
make lint              # golangci-lint
make ci-check          # всё, что проверяет CI на стадиях lint+test
make generate-api      # сгенерировать обвязку из api/openapi.yaml
make migrate-create NAME=add_users
```
