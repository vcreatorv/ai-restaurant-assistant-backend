package http

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// UserHandler HTTP-делирий для user-фичи
type UserHandler struct {
	usecase user.Usecase
}

// New создаёт UserHandler
func New(uc user.Usecase) UserHandler {
	return UserHandler{usecase: uc}
}
