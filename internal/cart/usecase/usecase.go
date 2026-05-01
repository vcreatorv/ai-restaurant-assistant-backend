// Package usecase реализует cart.Usecase.
package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
)

// Deps зависимости cart.Usecase
type Deps struct {
	// Repo репозиторий корзин
	Repo cart.Repository
	// Menu menu-фасад (для GetDishesByIDs при сборке CartView)
	Menu menu.Usecase
}

type cartUsecase struct {
	repo cart.Repository
	menu menu.Usecase
}

// New создаёт cart.Usecase
func New(d Deps) cart.Usecase {
	return &cartUsecase{
		repo: d.Repo,
		menu: d.Menu,
	}
}
