package auth

import (
	"context"
	"errors"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

var (
	// ErrInvalidCredentials неверная пара email/пароль или старый пароль
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrAlreadyRegistered текущая сессия уже привязана к customer/admin
	ErrAlreadyRegistered = errors.New("already registered")
)

// Usecase сценарии аутентификации
type Usecase interface {
	// Register регистрирует customer'а
	Register(
		ctx context.Context,
		sessionID uuid.UUID,
		currentUserID *uuid.UUID,
		email, password string,
	) (*usecasemodels.User, *usecasemodels.Session, error)
	// Login авторизует пользователя
	Login(
		ctx context.Context,
		sessionID uuid.UUID,
		email, password string,
	) (*usecasemodels.User, *usecasemodels.Session, error)
	// Logout удаляет сессию
	Logout(ctx context.Context, sessionID uuid.UUID) error
	// ChangePassword меняет пароль
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}
