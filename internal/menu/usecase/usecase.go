package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/cohere"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
)

// Deps зависимости menuUsecase.
//
// Cohere и Qdrant требуются только для админ-инструментов embed-preview и
// debug-search; в тестовом или dev-окружении без RAG можно оставить nil —
// соответствующие методы вернут ошибку «индексер не настроен».
type Deps struct {
	Repo    menu.Repository
	Storage s3.Storage
	Indexer indexer.Indexer
	Cohere  *cohere.Client
	Qdrant  *qdrant.Client
}

type menuUsecase struct {
	repo    menu.Repository
	storage s3.Storage
	// indexer переиндексирует блюда в Qdrant после CRUD (если изменились embed/payload-поля).
	// nil допустимо — usecase будет работать без переиндексации (полезно для тестов и режима без RAG).
	indexer indexer.Indexer
	// cohere / qdrant нужны админ-эндпоинтам embed-preview и debug-search.
	// nil допустимо — методы аккуратно вернут ErrIndexerNotConfigured.
	cohere *cohere.Client
	qdrant *qdrant.Client
}

// New создаёт menu.Usecase.
//
// d.Indexer / d.Cohere / d.Qdrant могут быть nil — это просто отключит
// автопереиндексацию и admin-инструменты embed-preview / debug-search.
// На проде все три должны быть выставлены.
func New(d Deps) menu.Usecase {
	return &menuUsecase{
		repo:    d.Repo,
		storage: d.Storage,
		indexer: d.Indexer,
		cohere:  d.Cohere,
		qdrant:  d.Qdrant,
	}
}
