package session

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

// ErrNotFound сессия не найдена
var ErrNotFound = errors.New("session not found")

// Usecase сценарии работы с сессией
type Usecase interface {
	// Ensure возвращает существующую сессию или создаёт новую анонимную
	Ensure(ctx context.Context, id *uuid.UUID) (*usecasemodels.Session, error)
	// AttachUser привязывает сессию к пользователю и ротирует CSRF
	AttachUser(ctx context.Context, sessionID, userID uuid.UUID) (*usecasemodels.Session, error)
	// Destroy удаляет сессию
	Destroy(ctx context.Context, sessionID uuid.UUID) error
}

// Repository хранилище сессий
type Repository interface {
	// Load возвращает сессию по идентификатору
	Load(ctx context.Context, id uuid.UUID) (*repositorymodels.Session, error)
	// Save сохраняет сессию с TTL
	Save(ctx context.Context, id uuid.UUID, s *repositorymodels.Session) error
	// Delete удаляет сессию
	Delete(ctx context.Context, id uuid.UUID) error
}
