package http

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// AuthHandler HTTP-делирий для auth-фичи
type AuthHandler struct {
	auth auth.Usecase
	user user.Usecase
}

// New создаёт AuthHandler
func New(authUC auth.Usecase, userUC user.Usecase) AuthHandler {
	return AuthHandler{auth: authUC, user: userUC}
}
