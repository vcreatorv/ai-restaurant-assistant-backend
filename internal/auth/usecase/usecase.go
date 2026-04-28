package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/bcrypt"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/uuid"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

type authUsecase struct {
	users   user.Repository
	session session.Usecase
	hasher  bcrypt.Hasher
	uuid    uuid.Generator
}

// New создаёт auth.Usecase
func New(
	users user.Repository,
	session session.Usecase,
	hasher bcrypt.Hasher,
	uuidGen uuid.Generator,
) auth.Usecase {
	return &authUsecase{
		users:   users,
		session: session,
		hasher:  hasher,
		uuid:    uuidGen,
	}
}
