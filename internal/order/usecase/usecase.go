// Package usecase реализует order.Usecase.
package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/order"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// Deps зависимости order.Usecase
type Deps struct {
	// Repo репозиторий заказов
	Repo order.Repository
	// CartRepo репозиторий корзин (для чтения cart_items при checkout и очистки после)
	CartRepo cart.Repository
	// Menu menu-фасад (snapshot name/price + проверка is_available)
	Menu menu.Usecase
	// Users user-фасад (snapshot контактов гостя)
	Users user.Usecase
	// UUID генератор UUID для order-id
	UUID order.UUIDGen
}

type orderUsecase struct {
	repo     order.Repository
	cartRepo cart.Repository
	menu     menu.Usecase
	users    user.Usecase
	uuid     order.UUIDGen
}

// New создаёт order.Usecase
func New(d Deps) order.Usecase {
	return &orderUsecase{
		repo:     d.Repo,
		cartRepo: d.CartRepo,
		menu:     d.Menu,
		users:    d.Users,
		uuid:     d.UUID,
	}
}
