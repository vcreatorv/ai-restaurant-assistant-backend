package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/cart"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"

	"github.com/google/uuid"
)

// currencyDefault код валюты на MVP — единственный для всех блюд.
// Если в будущем появятся блюда с другой валютой, считать total с агрегацией не получится
// и логику придётся пересмотреть; пока единая RUB.
const currencyDefault = "RUB"

// Get возвращает текущую корзину пользователя; создаёт пустую если её нет
func (uc *cartUsecase) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*usecasemodels.CartView, error) {
	c, err := uc.repo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find or create cart: %w", err)
	}
	return uc.buildView(ctx, c)
}

// AddItem добавляет позицию в корзину (или увеличивает quantity, если позиция уже есть).
// Проверяет: блюдо существует, is_available=true, итоговый quantity в [1, QuantityMax].
func (uc *cartUsecase) AddItem(
	ctx context.Context,
	userID uuid.UUID,
	req usecasemodels.CartItemAdd,
) (*usecasemodels.CartView, error) {
	if req.Quantity < cart.QuantityMin || req.Quantity > cart.QuantityMax {
		return nil, cart.ErrInvalidQuantity
	}
	dish, err := uc.menu.GetDish(ctx, req.DishID)
	if err != nil {
		if errors.Is(err, menu.ErrDishNotFound) {
			return nil, cart.ErrDishNotFound
		}
		return nil, fmt.Errorf("get dish: %w", err)
	}
	if !dish.IsAvailable {
		return nil, cart.ErrDishUnavailable
	}

	c, err := uc.repo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find or create cart: %w", err)
	}

	// Проверим, есть ли уже позиция: чтобы валидировать суммарный quantity до записи.
	existing, err := uc.repo.FindItem(ctx, c.ID, req.DishID)
	if err != nil && !errors.Is(err, cart.ErrCartItemNotFound) {
		return nil, fmt.Errorf("find item: %w", err)
	}
	totalQty := req.Quantity
	if existing != nil {
		totalQty += existing.Quantity
	}
	if totalQty > cart.QuantityMax {
		return nil, cart.ErrInvalidQuantity
	}

	if _, err := uc.repo.UpsertItem(ctx, c.ID, req.DishID, req.Quantity, req.Note); err != nil {
		return nil, fmt.Errorf("upsert item: %w", err)
	}
	return uc.buildView(ctx, c)
}

// PatchItem меняет quantity / note / sort_order у позиции.
// quantity передаётся ненулевым (>=1, <=50). quantity=0 не поддерживается:
// для удаления — отдельный endpoint DELETE.
func (uc *cartUsecase) PatchItem(
	ctx context.Context,
	userID uuid.UUID,
	dishID int,
	patch usecasemodels.CartItemPatch,
) (*usecasemodels.CartView, error) {
	if patch.Quantity != nil {
		q := *patch.Quantity
		if q < cart.QuantityMin || q > cart.QuantityMax {
			return nil, cart.ErrInvalidQuantity
		}
	}

	c, err := uc.repo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find or create cart: %w", err)
	}

	if patch.Quantity != nil {
		if _, qerr := uc.repo.SetItemQuantity(ctx, c.ID, dishID, *patch.Quantity); qerr != nil {
			return nil, qerr
		}
	}
	if patch.Note != nil || patch.SortOrder != nil {
		if _, perr := uc.repo.PatchItemFields(ctx, c.ID, dishID, patch.Note, patch.SortOrder); perr != nil {
			return nil, perr
		}
	}
	return uc.buildView(ctx, c)
}

// RemoveItem удаляет позицию по dish_id
func (uc *cartUsecase) RemoveItem(ctx context.Context, userID uuid.UUID, dishID int) error {
	c, err := uc.repo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("find or create cart: %w", err)
	}
	return uc.repo.DeleteItem(ctx, c.ID, dishID)
}

// Clear удаляет все позиции корзины
func (uc *cartUsecase) Clear(ctx context.Context, userID uuid.UUID) error {
	c, err := uc.repo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("find or create cart: %w", err)
	}
	return uc.repo.DeleteAllItems(ctx, c.ID)
}

// buildView собирает CartView: подгружает блюда, считает total по доступным позициям,
// формирует warnings для блюд из стоп-листа.
func (uc *cartUsecase) buildView(
	ctx context.Context,
	c *repositorymodels.Cart,
) (*usecasemodels.CartView, error) {
	items, err := uc.repo.ListItems(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	if len(items) == 0 {
		return &usecasemodels.CartView{
			Items:    []usecasemodels.CartItemView{},
			Currency: currencyDefault,
			Warnings: []usecasemodels.CartWarning{},
		}, nil
	}

	dishIDs := make([]int, 0, len(items))
	for _, it := range items {
		dishIDs = append(dishIDs, it.DishID)
	}
	dishes, err := uc.menu.GetDishesByIDs(ctx, dishIDs)
	if err != nil {
		return nil, fmt.Errorf("get dishes: %w", err)
	}
	dishesByID := make(map[int]usecasemodels.Dish, len(dishes))
	for _, d := range dishes {
		dishesByID[d.ID] = d
	}

	out := &usecasemodels.CartView{
		Items:    make([]usecasemodels.CartItemView, 0, len(items)),
		Currency: currencyDefault,
		Warnings: []usecasemodels.CartWarning{},
	}
	totalMinor := 0
	for _, it := range items {
		dish, ok := dishesByID[it.DishID]
		if !ok {
			// Блюдо удалено физически (теоретически невозможно — у нас FK RESTRICT,
			// но на всякий случай: пометим warning и пропустим).
			out.Warnings = append(out.Warnings, usecasemodels.CartWarning{
				Code:   usecasemodels.CartWarningDishUnavailable,
				DishID: it.DishID,
			})
			continue
		}
		if !dish.IsAvailable {
			out.Warnings = append(out.Warnings, usecasemodels.CartWarning{
				Code:   usecasemodels.CartWarningDishUnavailable,
				DishID: it.DishID,
			})
		}
		line := dish.PriceMinor * it.Quantity
		out.Items = append(out.Items, usecasemodels.CartItemView{
			DishID:         it.DishID,
			Name:           dish.Name,
			PriceMinor:     dish.PriceMinor,
			Quantity:       it.Quantity,
			Note:           it.Note,
			SortOrder:      it.SortOrder,
			AddedAt:        it.AddedAt,
			Available:      dish.IsAvailable,
			LineTotalMinor: line,
		})
		if dish.IsAvailable {
			totalMinor += line
		}
	}
	out.TotalMinor = totalMinor
	return out, nil
}
