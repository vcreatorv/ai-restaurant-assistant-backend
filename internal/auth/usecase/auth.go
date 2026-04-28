package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/bcrypt"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
	"github.com/google/uuid"
)

// Register регистрирует customer'а
func (uc *authUsecase) Register(
	ctx context.Context,
	sessionID uuid.UUID,
	currentUserID *uuid.UUID,
	email, password string,
) (*usecasemodels.User, *usecasemodels.Session, error) {
	if err := uc.ensureEmailFree(ctx, email); err != nil {
		return nil, nil, err
	}

	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := uc.upsertCustomer(ctx, currentUserID, email, hash)
	if err != nil {
		return nil, nil, err
	}

	s, err := uc.session.AttachUser(ctx, sessionID, u.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("attach session: %w", err)
	}
	return u, s, nil
}

// Login авторизует пользователя
func (uc *authUsecase) Login(
	ctx context.Context,
	sessionID uuid.UUID,
	email, password string,
) (*usecasemodels.User, *usecasemodels.Session, error) {
	raw, err := uc.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, nil, auth.ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("find by email: %w", err)
	}
	u := usecasemodels.UserFromRepository(raw)

	if cmpErr := uc.hasher.Compare(u.PasswordHash, password); cmpErr != nil {
		if errors.Is(cmpErr, bcrypt.ErrMismatch) {
			return nil, nil, auth.ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("compare password: %w", cmpErr)
	}

	s, err := uc.session.AttachUser(ctx, sessionID, u.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("attach session: %w", err)
	}
	return u, s, nil
}

// Logout удаляет сессию
func (uc *authUsecase) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return uc.session.Destroy(ctx, sessionID)
}

// ChangePassword меняет пароль
func (uc *authUsecase) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword, newPassword string,
) error {
	raw, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	u := usecasemodels.UserFromRepository(raw)
	if u.IsGuest() {
		return user.ErrInsufficientRole
	}

	if cmpErr := uc.hasher.Compare(u.PasswordHash, currentPassword); cmpErr != nil {
		if errors.Is(cmpErr, bcrypt.ErrMismatch) {
			return auth.ErrInvalidCredentials
		}
		return fmt.Errorf("compare password: %w", cmpErr)
	}

	hash, err := uc.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	u.PasswordHash = hash

	if err := uc.users.Update(ctx, usecasemodels.UserToRepository(u)); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (uc *authUsecase) ensureEmailFree(ctx context.Context, email string) error {
	_, err := uc.users.FindByEmail(ctx, email)
	if err == nil {
		return user.ErrEmailTaken
	}
	if errors.Is(err, user.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("find by email: %w", err)
}

func (uc *authUsecase) upsertCustomer(
	ctx context.Context,
	currentUserID *uuid.UUID,
	email, hash string,
) (*usecasemodels.User, error) {
	if currentUserID != nil {
		raw, err := uc.users.FindByID(ctx, *currentUserID)
		if err != nil && !errors.Is(err, user.ErrNotFound) {
			return nil, fmt.Errorf("load current user: %w", err)
		}
		if err == nil {
			u := usecasemodels.UserFromRepository(raw)
			if !u.IsGuest() {
				return nil, auth.ErrAlreadyRegistered
			}
			u.Email = email
			u.PasswordHash = hash
			u.Role = usecasemodels.RoleCustomer
			if err := uc.users.Update(ctx, usecasemodels.UserToRepository(u)); err != nil {
				return nil, fmt.Errorf("update user: %w", err)
			}
			return u, nil
		}
	}

	u := &usecasemodels.User{
		ID:           uc.uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Role:         usecasemodels.RoleCustomer,
		Allergens:    []string{},
		Dietary:      []string{},
	}
	if err := uc.users.Create(ctx, usecasemodels.UserToRepository(u)); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}
