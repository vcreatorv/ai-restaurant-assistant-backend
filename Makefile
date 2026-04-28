.PHONY: help build run test test-integration lint fmt group-imports tidy generate-api \
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
	python scripts/group-imports.py $$(find . -name '*.go' -not -path './api/generated/*')

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

group-imports-check:
	python scripts/group-imports.py --check $$(find . -name '*.go' -not -path './api/generated/*')
