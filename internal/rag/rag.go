// Package rag описывает конфигурацию RAG-pipeline (Cohere + Qdrant).
package rag

import "time"

// Config параметры RAG
type Config struct {
	// Cohere параметры клиента Cohere (embeddings + rerank)
	Cohere CohereConfig `yaml:"cohere"`
	// Qdrant параметры клиента Qdrant (векторное хранилище)
	Qdrant QdrantConfig `yaml:"qdrant"`
	// LLM основной LLM-провайдер для генерации ответа (OpenRouter или NVIDIA NIM).
	LLM LLMConfig `yaml:"llm"`
	// Classifier отдельный LLM-провайдер для классификатора намерений.
	// Если Classifier.Provider == "" — используется LLM (тот же клиент, что и для основного ответа).
	// Это позволяет посадить классификатор на более лёгкую/дешёвую модель,
	// сохранив основную качественную для самого ответа.
	Classifier LLMConfig `yaml:"classifier"`
	// Search параметры стадии retrieval
	Search SearchConfig `yaml:"search"`
	// Chat параметры RAG-стороны чата
	Chat ChatRAGConfig `yaml:"chat"`
}

// LLMConfig переключатель между LLM-провайдерами + общие параметры
type LLMConfig struct {
	// Provider активный провайдер: "openrouter" | "nvidia" | "gigachat"
	Provider string `yaml:"provider"`
	// Common параметры, общие для всех провайдеров (temperature, max_tokens, timeouts)
	Common LLMCommonConfig `yaml:"common"`
	// OpenRouter параметры провайдера OpenRouter
	OpenRouter OpenRouterConfig `yaml:"openrouter"`
	// Nvidia параметры провайдера NVIDIA NIM (build.nvidia.com)
	Nvidia NvidiaConfig `yaml:"nvidia"`
	// GigaChat параметры провайдера Сбера (gigachat.devices.sberbank.ru)
	GigaChat GigaChatConfig `yaml:"gigachat"`
}

// LLMCommonConfig общие параметры запроса, не зависящие от провайдера
type LLMCommonConfig struct {
	// Temperature сэмплинг-температура 0..1
	Temperature float64 `yaml:"temperature"`
	// MaxTokens максимум выходных токенов
	MaxTokens int `yaml:"max_tokens"`
	// RequestTimeout таймаут на весь HTTP-запрос (включая стрим)
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// FirstTokenTimeout таймаут до получения первого токена в SSE-стриме
	FirstTokenTimeout time.Duration `yaml:"first_token_timeout"`
}

// CohereConfig параметры клиента Cohere
type CohereConfig struct {
	// BaseURL базовый URL API (https://api.cohere.com)
	BaseURL string `yaml:"base_url"`
	// APIKey ключ API; в норме переопределяется COHERE_API_KEY
	APIKey string `yaml:"api_key"`
	// EmbedModel имя модели эмбеддинга (embed-multilingual-v3.0)
	EmbedModel string `yaml:"embed_model"`
	// EmbedDim размерность вектора (для embed-multilingual-v3.0 — 1024)
	EmbedDim int `yaml:"embed_dim"`
	// RerankModel имя модели рерэнкинга (rerank-multilingual-v3.0)
	RerankModel string `yaml:"rerank_model"`
	// RequestTimeout таймаут одного HTTP-запроса
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// EmbedBatchSize максимальное количество текстов в одном вызове Embed
	EmbedBatchSize int `yaml:"embed_batch_size"`
	// RetryAttempts количество попыток одного запроса (1 = без повторов)
	RetryAttempts int `yaml:"retry_attempts"`
	// RetryDelay пауза между попытками
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// QdrantConfig параметры клиента Qdrant
type QdrantConfig struct {
	// URL базовый URL HTTP API (http://localhost:6333)
	URL string `yaml:"url"`
	// APIKey ключ API; переопределяется QDRANT_API_KEY
	APIKey string `yaml:"api_key"`
	// RequestTimeout таймаут одного HTTP-запроса
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// Collection имя коллекции для блюд
	Collection string `yaml:"collection"`
	// Distance метрика расстояния: Cosine, Dot, Euclid
	Distance string `yaml:"distance"`
	// HNSW параметры HNSW-индекса
	HNSW HNSWConfig `yaml:"hnsw"`
	// UpsertBatchSize размер батча для одной операции upsert
	UpsertBatchSize int `yaml:"upsert_batch_size"`
}

// HNSWConfig параметры HNSW-индекса Qdrant
type HNSWConfig struct {
	// M максимальное число соседей в HNSW-графе
	M int `yaml:"m"`
	// EfConstruct ширина beam search при построении графа
	EfConstruct int `yaml:"ef_construct"`
}

// OpenRouterConfig параметры провайдера OpenRouter (https://openrouter.ai)
type OpenRouterConfig struct {
	// BaseURL базовый URL API (без хвостового слэша, без /chat/completions)
	BaseURL string `yaml:"base_url"`
	// APIKey ключ API; переопределяется OPENROUTER_API_KEY
	APIKey string `yaml:"api_key"`
	// Model имя модели (например openrouter/free, meta-llama/llama-3.3-70b-instruct:free)
	Model string `yaml:"model"`
	// Referer значение HTTP-Referer (опц., для аналитики OpenRouter)
	Referer string `yaml:"referer"`
	// Title значение X-Title (опц., для аналитики OpenRouter)
	Title string `yaml:"title"`
}

// NvidiaConfig параметры провайдера NVIDIA NIM (build.nvidia.com / integrate.api.nvidia.com)
type NvidiaConfig struct {
	// BaseURL базовый URL API (https://integrate.api.nvidia.com/v1)
	BaseURL string `yaml:"base_url"`
	// APIKey ключ API (nvapi-...); переопределяется NVIDIA_API_KEY
	APIKey string `yaml:"api_key"`
	// Model имя модели (например meta/llama-3.3-70b-instruct, qwen/qwen2.5-72b-instruct)
	Model string `yaml:"model"`
}

// GigaChatConfig параметры провайдера GigaChat от Сбера.
//
// Аутентификация двухступенчатая: AuthorizationKey (base64 от client_id:client_secret,
// выдаётся в личном кабинете developers.sber.ru) меняется на короткоживущий access_token
// (~30 минут) через OAuth endpoint AuthURL. Клиент сам обновляет токен по мере истечения.
//
// TLS — через корневые сертификаты Минцифры (см. certs/russian_trusted_bundle.pem):
// домены *.devices.sberbank.ru подписаны Минцифры, которой нет в дефолтных trust store.
type GigaChatConfig struct {
	// BaseURL базовый URL API чата (https://gigachat.devices.sberbank.ru/api/v1)
	BaseURL string `yaml:"base_url"`
	// AuthURL endpoint выдачи OAuth-токена (https://ngw.devices.sberbank.ru:9443/api/v2/oauth)
	AuthURL string `yaml:"auth_url"`
	// AuthorizationKey base64-строка client_id:client_secret из ЛК; переопределяется GIGACHAT_AUTHORIZATION_KEY
	AuthorizationKey string `yaml:"authorization_key"`
	// Scope область доступа: GIGACHAT_API_PERS (бесплатный для физлиц) | GIGACHAT_API_B2B | GIGACHAT_API_CORP
	Scope string `yaml:"scope"`
	// Model имя модели (GigaChat | GigaChat-Pro | GigaChat-Max)
	Model string `yaml:"model"`
	// CABundlePath путь к PEM-bundle корневых сертификатов Минцифры. Относительный путь
	// разрешается от рабочей директории процесса (в docker — /app).
	CABundlePath string `yaml:"ca_bundle_path"`
}

// SearchConfig параметры стадии retrieval
type SearchConfig struct {
	// TopK сколько кандидатов берём из Qdrant до рерэнкера
	TopK int `yaml:"top_k"`
	// RerankTopN сколько кандидатов остаётся после рерэнкера
	RerankTopN int `yaml:"rerank_top_n"`
	// RerankMinScore минимальный score рерэнкера; ниже — отбрасываем
	RerankMinScore float64 `yaml:"rerank_min_score"`
}

// ChatRAGConfig RAG-параметры чата.
//
// Списки main/companion-категорий живут в БД (поле categories.role); см. миграцию
// 000009_categories_role.up.sql. В конфиге остались только числовые пороги алгоритма.
type ChatRAGConfig struct {
	// HistoryRecentPairs сколько последних пар (user, assistant) подаём в LLM
	// в дополнение к anchor-сообщению (первый user-msg чата).
	HistoryRecentPairs int `yaml:"history_recent_pairs"`
	// MainMinCategories порог количества уникальных main-категорий в reranked top-N;
	// если покрыто меньше — диверсифицируем добавлением top-1 из непокрытых.
	MainMinCategories int `yaml:"main_min_categories"`
	// MainMaxAdded сколько максимум блюд можно добавить через диверсификацию main.
	MainMaxAdded int `yaml:"main_max_added"`
	// MainDiversifyMinScore минимальный cosine-score Qdrant для добавления
	// блюда из непокрытой main-категории; ниже — категория не релевантна запросу
	// (например, на «хочу пиццу» score супа будет ~0.3, и мы его не добавим).
	MainDiversifyMinScore float64 `yaml:"main_diversify_min_score"`
}

// CohereInputType тип входа для Cohere Embed: search_document для индексации, search_query для поиска
type CohereInputType string

const (
	// CohereInputDocument эмбеддинг документа (для индексации)
	CohereInputDocument CohereInputType = "search_document"
	// CohereInputQuery эмбеддинг запроса (для поиска)
	CohereInputQuery CohereInputType = "search_query"
)
