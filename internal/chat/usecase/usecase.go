// Package usecase реализует chat.Usecase.
package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
)

type chatUsecase struct {
	repo chat.Repository
	uuid chat.UUIDGen
	cfg  chat.UsecaseConfig
}

// New создаёт chat.Usecase
func New(repo chat.Repository, uuid chat.UUIDGen, cfg chat.UsecaseConfig) chat.Usecase {
	return &chatUsecase{repo: repo, uuid: uuid, cfg: cfg}
}
