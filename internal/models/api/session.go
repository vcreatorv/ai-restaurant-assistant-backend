package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
)

// SessionInfoFromUsecase маппит usecase.Session и usecase.User в api.SessionInfo
func SessionInfoFromUsecase(s *usecasemodels.Session, user *usecasemodels.User) SessionInfo {
	info := SessionInfo{
		Csrf:    s.CSRF,
		IsGuest: user == nil || user.IsGuest(),
	}
	if s.UserID != nil {
		u := *s.UserID
		info.UserId = &u
	}
	if user != nil {
		role := SessionInfoRole(string(user.Role))
		info.Role = &role
	}
	return info
}
