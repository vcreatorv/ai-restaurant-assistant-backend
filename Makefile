.PHONY: help build run test test-integration lint fmt group-imports tidy generate-api seed seed-descriptions embed-menu \
        wipe-menu reset-menu \
        migrate-up migrate-down migrate-create \
        docker-build docker-up docker-down docker-logs \
        ci-check fmt-check group-imports-check

help:
	@echo "Targets:"
	@echo "  build              - go build"
	@echo "  run                - запустить локальный бинарь"
	@echo "  test               - unit-тесты"
	@echo "  test-integration   - интеграционные тесты (build tag integration)"
	@echo "  lint               - golangci-lint"
	@echo "  fmt                - gofmt + goimports"
	@echo "  group-imports      - группировка импортов через scripts/group-imports.py"
	@echo "  tidy               - go mod tidy + verify"
	@echo "  generate-api       - сгенерировать код из OpenAPI"
	@echo "  seed               - залить меню (seed/menu.json) в PG + картинки в MinIO"
	@echo "  seed-descriptions  - обновить только description у блюд из seed/menu.json"
	@echo "  embed-menu         - проиндексировать блюда в Qdrant (Cohere embeddings)"
	@echo "  wipe-menu          - удалить всё меню: PG-таблицы меню, MinIO dishes/*, Qdrant collection"
	@echo "  reset-menu         - wipe-menu + seed + embed-menu (полная перезаливка из seed/menu.json)"
	@echo "  migrate-up         - применить миграции"
	@echo "  migrate-down       - откатить последнюю миграцию"
	@echo "  migrate-create     - создать новую миграцию (NAME=имя)"
	@echo "  docker-build       - собрать образ"
	@echo "  docker-up          - поднять стек локально"
	@echo "  docker-down        - остановить стек"
	@echo "  docker-logs        - логи app сервиса"
	@echo "  ci-check           - всё, что гонит CI на стадии lint+test"

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X main.Version=$(VERSION) \
  -X main.Commit=$(COMMIT) \
  -X main.BuildDate=$(BUILD_DATE)

DB_DSN ?= postgres://app:app@localhost:5432/app?sslmode=disable

# ----- Go -----

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/app ./cmd/app

run: build
	./bin/app

test:
	go test ./... -race -short

test-integration:
	go test ./... -tags=integration -race

lint:
	golangci-lint run --timeout 5m

fmt:
	gofmt -w .
	goimports -w .

group-imports:
	find . -name '*.go' \
	  -not -path './cmd/app/app/v1/*' \
	  -not -path './internal/models/api/api.gen.go' \
	  -not -path './bin/*' \
	  -print0 | xargs -0 -n1 python scripts/group-imports.py

tidy:
	go mod tidy
	go mod verify

# ----- OpenAPI -----

generate-api: gen-types gen-server

gen-types:
	mkdir -p internal/models/api
	oapi-codegen -config api/types.cfg.yaml api/openapi.yaml

gen-server:
	mkdir -p cmd/app/app/v1
	oapi-codegen -config api/server.cfg.yaml api/openapi.yaml

# ----- Seed -----

# Запускать с хоста: переменные берутся из .env, PG/MinIO открыты на localhost.
# Перед запуском: make docker-up (нужны postgres/minio со сделанными миграциями).
seed:
	@bash -c 'set -a && . ./.env && set +a && \
	  POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:5432/$$POSTGRES_DB?sslmode=disable" \
	  S3_ENDPOINT=localhost:$${MINIO_API_PORT:-9000} \
	  S3_ACCESS_KEY=$$MINIO_ROOT_USER \
	  S3_SECRET_KEY=$$MINIO_ROOT_PASSWORD \
	  S3_BUCKET=$$MINIO_BUCKET \
	  S3_PUBLIC_BASE_URL=http://localhost:$${MINIO_API_PORT:-9000}/$$MINIO_BUCKET \
	  go run ./cmd/seed -config configs/config.yaml -seed seed/menu.json -assets seed/assets'

# Индексирует все блюда из PG в Qdrant. Идемпотентно: повторный запуск перезапишет.
# Перед запуском: make docker-up (нужны postgres/qdrant) + COHERE_API_KEY в .env.
embed-menu:
	@bash -c 'set -a && . ./.env && set +a && \
	  POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:5432/$$POSTGRES_DB?sslmode=disable" \
	  S3_ENDPOINT=localhost:$${MINIO_API_PORT:-9000} \
	  S3_ACCESS_KEY=$$MINIO_ROOT_USER \
	  S3_SECRET_KEY=$$MINIO_ROOT_PASSWORD \
	  S3_BUCKET=$$MINIO_BUCKET \
	  S3_PUBLIC_BASE_URL=http://localhost:$${MINIO_API_PORT:-9000}/$$MINIO_BUCKET \
	  QDRANT_URL=http://localhost:$${QDRANT_HTTP_PORT:-6333} \
	  QDRANT_API_KEY=$$QDRANT_API_KEY \
	  COHERE_API_KEY=$$COHERE_API_KEY \
	  go run ./cmd/embed-menu -config configs/config.yaml'

# Обновляет только description у уже существующих блюд по name. Картинки/S3 не трогаются.
# S3-env пробрасываются ради валидации конфига, сам S3-клиент в этом режиме не создаётся.
seed-descriptions:
	@bash -c 'set -a && . ./.env && set +a && \
	  POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:5432/$$POSTGRES_DB?sslmode=disable" \
	  S3_ENDPOINT=localhost:$${MINIO_API_PORT:-9000} \
	  S3_ACCESS_KEY=$$MINIO_ROOT_USER \
	  S3_SECRET_KEY=$$MINIO_ROOT_PASSWORD \
	  S3_BUCKET=$$MINIO_BUCKET \
	  S3_PUBLIC_BASE_URL=http://localhost:$${MINIO_API_PORT:-9000}/$$MINIO_BUCKET \
	  go run ./cmd/seed -config configs/config.yaml -seed seed/menu.json -mode=update-descriptions'

# Полностью сбрасывает меню во всех трёх хранилищах:
#   PG: TRUNCATE dish_tags, dishes, tags, categories (RESTART IDENTITY CASCADE)
#   MinIO: rm -r dishes/ из бакета (id-image_url'ы protected картинок не трогаем —
#          но т.к. PG обнулён, ссылки в любом случае пересоздадутся при seed)
#   Qdrant: DELETE /collections/dishes
# Сохранность: пользователи, чаты, сессии не затрагиваются. Только меню.
# Идемпотентно: если что-то уже отсутствует (например, Qdrant collection не создана),
# таргет всё равно завершится успешно.
wipe-menu:
	@bash -c 'set -a && . ./.env && set +a && \
	  echo "=== PG: truncate menu tables ===" && \
	  docker compose exec -T postgres psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -c \
	    "TRUNCATE TABLE dish_tags, dishes, tags, categories RESTART IDENTITY CASCADE;" && \
	  echo "=== MinIO: rm -r dishes/ ===" && \
	  docker compose exec -T minio-init mc alias set local http://minio:9000 "$$MINIO_ROOT_USER" "$$MINIO_ROOT_PASSWORD" >/dev/null && \
	  docker compose exec -T minio-init mc rm --recursive --force "local/$$MINIO_BUCKET/dishes" || true && \
	  echo "=== Qdrant: drop dishes collection ===" && \
	  curl -sS -X DELETE "http://localhost:$${QDRANT_HTTP_PORT:-6333}/collections/dishes" \
	    -H "api-key: $$QDRANT_API_KEY" -o /dev/null -w "  status: %{http_code}\n" && \
	  echo "=== wipe-menu done ==="'

# Полная перезаливка меню из seed/menu.json. После этого приложение готово к работе.
reset-menu: wipe-menu seed embed-menu
	@echo "=== reset-menu done ==="

# ----- Migrations -----

migrate-up:
	migrate -path migrations -database "$(DB_DSN)" up

migrate-down:
	migrate -path migrations -database "$(DB_DSN)" down 1

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "usage: make migrate-create NAME=add_users"; exit 1; fi
	migrate create -ext sql -dir migrations -seq $(NAME)

# ----- Docker -----

docker-build:
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILD_DATE=$(BUILD_DATE) \
	  -t ai-restaurant-assistant:local .

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

# ----- CI -----

ci-check: fmt-check group-imports-check lint test

fmt-check:
	@test -z "$$(gofmt -l . | tee /dev/stderr)"
	@test -z "$$(goimports -l . | tee /dev/stderr)"

group-imports-check: group-imports
	@out=$$(git diff --name-only); \
	if [ -n "$$out" ]; then \
	  echo "import groups drifted:"; echo "$$out"; \
	  git diff; exit 1; \
	fi
