package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
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

// UpdateProfile обновляет профиль
func (uc *userUsecase) UpdateProfile(
	ctx context.Context,
	userID uuid.UUID,
	patch usecasemodels.ProfilePatch,
) (*usecasemodels.User, error) {
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
