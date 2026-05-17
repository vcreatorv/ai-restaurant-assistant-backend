package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu/indexer"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
)

type menuUsecase struct {
	repo    menu.Repository
	storage s3.Storage
	// indexer переиндексирует блюда в Qdrant после CRUD (если изменились embed/payload-поля).
	// nil допустимо — usecase будет работать без переиндексации (полезно для тестов и режима без RAG).
	indexer indexer.Indexer
}

// New создаёт menu.Usecase.
//
// Параметр idx может быть nil — в этом случае автопереиндексация выключена.
// На проде должен быть передан реальный индексер, иначе после правки блюда в админке
// Qdrant будет отдавать устаревшие вектора/payload, пока не запустят make embed-menu вручную.
func New(repo menu.Repository, storage s3.Storage, idx indexer.Indexer) menu.Usecase {
	return &menuUsecase{repo: repo, storage: storage, indexer: idx}
}
