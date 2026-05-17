package usecase

import (
	"context"
	"fmt"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
	"github.com/google/uuid"
)

// GetByID возвращает пользователя по идентификатору
func (uc *userUsecase) GetByID(ctx context.Context, id uuid.UUID) (*usecasemodels.User, error) {
	raw, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return usecasemodels.UserFromRepository(raw), nil
}

// GetProfile возвращает профиль customer'а или admin'а
func (uc *userUsecase) GetProfile(ctx context.Context, userID uuid.UUID) (*usecasemodels.User, error) {
	u, err := uc.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.IsGuest() {
		return nil, user.ErrInsufficientRole
	}
	return u, nil
}

// UpdateProfile обновляет профиль.
//
// Валидация allergens/dietary по whitelist кодов критична для безопасности:
// если в users.allergens попадёт значение, которого нет в dishes.allergens
// (например, локализованная строка вместо кода), Qdrant-фильтр must_not
// просто не сматчит ничего и пропустит запрещённые блюда. CHECK constraint
// в БД дублирует это правило на случай прямой записи в обход usecase.
func (uc *userUsecase) UpdateProfile(
	ctx context.Context,
	userID uuid.UUID,
	patch usecasemodels.ProfilePatch,
) (*usecasemodels.User, error) {
	if patch.Allergens != nil {
		for _, a := range *patch.Allergens {
			if !user.IsAllowedAllergen(a) {
				return nil, fmt.Errorf("%w: %q", user.ErrInvalidAllergen, a)
			}
		}
	}
	if patch.Dietary != nil {
		for _, d := range *patch.Dietary {
			if !user.IsAllowedDietary(d) {
				return nil, fmt.Errorf("%w: %q", user.ErrInvalidDietary, d)
			}
		}
	}
	u, err := uc.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.IsGuest() {
		return nil, user.ErrInsufficientRole
	}
	applyPatch(u, patch)
	if err := uc.repo.Update(ctx, usecasemodels.UserToRepository(u)); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}

func applyPatch(u *usecasemodels.User, p usecasemodels.ProfilePatch) {
	if p.FirstName != nil {
		u.FirstName = *p.FirstName
	}
	if p.LastName != nil {
		u.LastName = *p.LastName
	}
	if p.Phone != nil {
		u.Phone = *p.Phone
	}
	if p.Allergens != nil {
		u.Allergens = *p.Allergens
	}
	if p.Dietary != nil {
		u.Dietary = *p.Dietary
	}
}
