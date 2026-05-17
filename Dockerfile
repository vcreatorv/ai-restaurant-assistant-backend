# syntax=docker/dockerfile:1.7

# ---------- builder ----------
FROM golang:1.25-alpine AS build

WORKDIR /src

# Кэшируем зависимости отдельным слоем
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w \
      -X main.Version=${VERSION} \
      -X main.Commit=${COMMIT} \
      -X main.BuildDate=${BUILD_DATE}" \
    -o /out/app ./cmd/app

# ---------- runtime ----------
FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/app /app
COPY --from=build /src/configs /configs
# Корневые сертификаты Минцифры (нужны для GigaChat TLS — Минцифры нет в дефолтном
# trust store distroless/alpine). Подгружаются только в internal/pkg/gigachat,
# другие HTTPS-вызовы (Cohere, NVIDIA, OpenRouter, MinIO, Qdrant) их не используют.
COPY --from=build /src/certs /certs

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app", "-config", "/configs/config.yaml"]
