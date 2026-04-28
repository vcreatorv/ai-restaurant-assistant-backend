package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/csrf"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/uuid"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
)

type sessionUsecase struct {
	repo session.Repository
	uuid uuid.Generator
	csrf csrf.Generator
}

// New создаёт session.Usecase
func New(repo session.Repository, uuidGen uuid.Generator, csrfGen csrf.Generator) session.Usecase {
	return &sessionUsecase{
		repo: repo,
		uuid: uuidGen,
		csrf: csrfGen,
	}
}
