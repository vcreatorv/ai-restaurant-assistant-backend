package user

import (
	"context"
	"errors"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

var (
	// ErrNotFound пользователь не найден
	ErrNotFound = errors.New("user not found")
	// ErrEmailTaken email уже занят
	ErrEmailTaken = errors.New("email already taken")
	// ErrInsufficientRole операция недоступна для текущей роли
	ErrInsufficientRole = errors.New("insufficient role")
	// ErrInvalidAllergen недопустимый код аллергена в профиле
	ErrInvalidAllergen = errors.New("invalid allergen code")
	// ErrInvalidDietary недопустимый код диетического предпочтения в профиле
	ErrInvalidDietary = errors.New("invalid dietary code")
)

// AllowedAllergens канонический whitelist кодов аллергенов.
//
// Значения должны совпадать с тем, что хранится в dishes.allergens
// (см. embed-menu и Qdrant payload). Любое расхождение между списком профиля
// и payload блюд приведёт к молчаливому пропуску фильтра must_not.
//
// При добавлении нового аллергена нужно одновременно:
//  1. расширить этот whitelist;
//  2. обновить CHECK constraint в БД (миграция);
//  3. обновить фронт-словарь { code → ruLabel }.
var AllowedAllergens = []string{
	"celery",
	"dairy",
	"eggs",
	"fish",
	"gluten",
	"mustard",
	"nuts",
	"peanuts",
	"sesame",
	"shellfish",
	"soy",
}

// AllowedDietary канонический whitelist кодов диетических предпочтений.
var AllowedDietary = []string{
	"vegetarian",
	"vegan",
	"halal",
	"kosher",
	"gluten_free",
	"lactose_free",
}

// IsAllowedAllergen проверяет, входит ли код в whitelist.
func IsAllowedAllergen(code string) bool {
	for _, a := range AllowedAllergens {
		if a == code {
			return true
		}
	}
	return false
}

// IsAllowedDietary проверяет, входит ли код в whitelist.
func IsAllowedDietary(code string) bool {
	for _, d := range AllowedDietary {
		if d == code {
			return true
		}
	}
	return false
}

// Usecase сценарии работы с пользователем
type Usecase interface {
	// GetByID возвращает пользователя по идентификатору
	GetByID(ctx context.Context, id uuid.UUID) (*usecasemodels.User, error)
	// GetProfile возвращает профиль customer'а или admin'а
	GetProfile(ctx context.Context, userID uuid.UUID) (*usecasemodels.User, error)
	// UpdateProfile обновляет профиль
	UpdateProfile(ctx context.Context, userID uuid.UUID, patch usecasemodels.ProfilePatch) (*usecasemodels.User, error)
}

// Repository хранилище пользователей
type Repository interface {
	// FindByID возвращает пользователя по идентификатору
	FindByID(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error)
	// FindByEmail возвращает пользователя по email
	FindByEmail(ctx context.Context, email string) (*repositorymodels.User, error)
	// Create вставляет нового пользователя
	Create(ctx context.Context, u *repositorymodels.User) error
	// Update сохраняет изменения пользователя
	Update(ctx context.Context, u *repositorymodels.User) error
}
