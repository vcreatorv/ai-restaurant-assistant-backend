package usecase

import (
	"context"
	"errors"
	"fmt"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
	"github.com/google/uuid"
)

// Ensure возвращает существующую сессию или создаёт новую анонимную
func (uc *sessionUsecase) Ensure(ctx context.Context, id *uuid.UUID) (*usecasemodels.Session, error) {
	if id != nil {
		s, err := uc.load(ctx, *id)
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, session.ErrNotFound) {
			return nil, fmt.Errorf("load session: %w", err)
		}
	}
	return uc.createAnonymous(ctx)
}

// AttachUser привязывает сессию к пользователю и ротирует CSRF
func (uc *sessionUsecase) AttachUser(ctx context.Context, sessionID, userID uuid.UUID) (*usecasemodels.Session, error) {
	s, err := uc.load(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	s.UserID = &userID
	s.RotateCSRF(uc.csrf.New())
	if err := uc.save(ctx, s); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	return s, nil
}

// Destroy удаляет сессию
func (uc *sessionUsecase) Destroy(ctx context.Context, sessionID uuid.UUID) error {
	if err := uc.repo.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (uc *sessionUsecase) load(ctx context.Context, id uuid.UUID) (*usecasemodels.Session, error) {
	raw, err := uc.repo.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return usecasemodels.SessionFromRepository(id, raw)
}

func (uc *sessionUsecase) save(ctx context.Context, s *usecasemodels.Session) error {
	return uc.repo.Save(ctx, s.ID, usecasemodels.SessionToRepository(s))
}

func (uc *sessionUsecase) createAnonymous(ctx context.Context) (*usecasemodels.Session, error) {
	s := &usecasemodels.Session{
		ID:   uc.uuid.New(),
		CSRF: uc.csrf.New(),
	}
	if err := uc.save(ctx, s); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	return s, nil
}
