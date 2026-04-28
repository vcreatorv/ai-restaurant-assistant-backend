package usecase

import (
	"fmt"

	"github.com/google/uuid"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
)

// Session сессия в доменной форме
type Session struct {
	// ID идентификатор сессии
	ID uuid.UUID
	// UserID идентификатор привязанного пользователя
	UserID *uuid.UUID
	// CSRF токен для защиты от CSRF
	CSRF string
}

// IsAnonymous проверяет, что сессия не привязана к пользователю
func (s *Session) IsAnonymous() bool { return s.UserID == nil }

// RotateCSRF меняет CSRF-токен
func (s *Session) RotateCSRF(csrf string) { s.CSRF = csrf }

// SessionFromRepository маппит repository-модель в usecase-модель
func SessionFromRepository(id uuid.UUID, r *repositorymodels.Session) (*Session, error) {
	if r == nil {
		return nil, fmt.Errorf("session repository model is nil")
	}
	s := &Session{ID: id, CSRF: r.CSRF}
	if r.UserID != nil {
		u, err := uuid.Parse(*r.UserID)
		if err != nil {
			return nil, fmt.Errorf("parse user_id: %w", err)
		}
		s.UserID = &u
	}
	return s, nil
}

// SessionToRepository маппит usecase-модель в repository-модель
func SessionToRepository(s *Session) *repositorymodels.Session {
	r := &repositorymodels.Session{CSRF: s.CSRF}
	if s.UserID != nil {
		v := s.UserID.String()
		r.UserID = &v
	}
	return r
}
