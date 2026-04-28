package user

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

var (
	// ErrNotFound пользователь не найден
	ErrNotFound = errors.New("user not found")
	// ErrEmailTaken email уже занят
	ErrEmailTaken = errors.New("email already taken")
	// ErrInsufficientRole операция недоступна для текущей роли
	ErrInsufficientRole = errors.New("insufficient role")
)

// Usecase сценарии работы с пользователем
type Usecase interface {
	// GetByID возвращает пользователя по идентификатору
	GetByID(ctx context.Context, id uuid.UUID) (*usecasemodels.User, error)
	// GetProfile возвращает профиль customer'а или admin'а
	GetProfile(ctx context.Context, userID uuid.UUID) (*usecasemodels.User, error)
	// UpdateProfile обновляет профиль
	UpdateProfile(ctx context.Context, userID uuid.UUID, patch usecasemodels.ProfilePatch) (*usecasemodels.User, error)
}

// Repository хранилище пользователей
type Repository interface {
	// FindByID возвращает пользователя по идентификатору
	FindByID(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error)
	// FindByEmail возвращает пользователя по email
	FindByEmail(ctx context.Context, email string) (*repositorymodels.User, error)
	// Create вставляет нового пользователя
	Create(ctx context.Context, u *repositorymodels.User) error
	// Update сохраняет изменения пользователя
	Update(ctx context.Context, u *repositorymodels.User) error
}
