package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

type userUsecase struct {
	repo user.Repository
}

// New создаёт user.Usecase
func New(repo user.Repository) user.Usecase {
	return &userUsecase{repo: repo}
}
