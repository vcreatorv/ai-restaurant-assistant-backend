package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
)

type menuUsecase struct {
	repo    menu.Repository
	storage s3.Storage
}

// New создаёт menu.Usecase
func New(repo menu.Repository, storage s3.Storage) menu.Usecase {
	return &menuUsecase{repo: repo, storage: storage}
}
