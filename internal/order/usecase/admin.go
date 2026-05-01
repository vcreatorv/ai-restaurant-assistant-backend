package usecase

import (
	"context"
	"fmt"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/order"

	"github.com/google/uuid"
)

// allowedStatusTransitions описывает state-machine заказа: from → set(to).
// Переходы намеренно линейные. cancelled — терминал, доступен из любого
// нетерминального; closed — терминал. Идентичный переход (X → X) не в карте.
var allowedStatusTransitions = map[usecasemodels.OrderStatus]map[usecasemodels.OrderStatus]struct{}{
	usecasemodels.OrderStatusAccepted: {
		usecasemodels.OrderStatusCooking:   {},
		usecasemodels.OrderStatusCancelled: {},
	},
	usecasemodels.OrderStatusCooking: {
		usecasemodels.OrderStatusReady:     {},
		usecasemodels.OrderStatusCancelled: {},
	},
	usecasemodels.OrderStatusReady: {
		// in_delivery — для fulfillment_type=delivery; closed — для pickup/dine_in.
		// Не различаем явно по fulfillment'у: admin сам выбирает корректный путь.
		usecasemodels.OrderStatusInDelivery: {},
		usecasemodels.OrderStatusClosed:     {},
		usecasemodels.OrderStatusCancelled:  {},
	},
	usecasemodels.OrderStatusInDelivery: {
		usecasemodels.OrderStatusClosed:    {},
		usecasemodels.OrderStatusCancelled: {},
	},
	// closed и cancelled — терминалы, переходов нет.
}

// AdminList возвращает заказы по фильтру (без ограничения по userID).
// userID может быть передан как явный фильтр в `userID` параметре.
func (uc *orderUsecase) AdminList(
	ctx context.Context,
	filter usecasemodels.OrderListFilter,
	userID *uuid.UUID,
) ([]usecasemodels.Order, int, error) {
	repoFilter := repositorymodels.OrderFilter{
		UserID: userID,
		From:   filter.From,
		To:     filter.To,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.Status != nil {
		s := string(*filter.Status)
		repoFilter.Status = &s
	}
	orders, total, err := uc.repo.ListOrders(ctx, repoFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	if len(orders) == 0 {
		return []usecasemodels.Order{}, total, nil
	}
	ids := make([]uuid.UUID, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
	}
	itemsByOrder, err := uc.repo.LoadOrderItems(ctx, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("load order items: %w", err)
	}
	out := make([]usecasemodels.Order, 0, len(orders))
	for i := range orders {
		out = append(out, *uc.assemble(&orders[i], itemsByOrder[orders[i].ID]))
	}
	return out, total, nil
}

// AdminGet возвращает заказ по id (без проверки владельца)
func (uc *orderUsecase) AdminGet(
	ctx context.Context,
	orderID uuid.UUID,
) (*usecasemodels.Order, error) {
	o, items, err := uc.repo.FindOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return uc.assemble(o, items), nil
}

// AdminUpdateStatus меняет статус с проверкой state-machine.
// Возможные ошибки:
//   - ErrInvalidStatus — если newStatus не из enum'а;
//   - ErrInvalidStatusTransition — если переход недопустим;
//   - ErrOrderNotFound — если заказа нет.
func (uc *orderUsecase) AdminUpdateStatus(
	ctx context.Context,
	orderID uuid.UUID,
	newStatus usecasemodels.OrderStatus,
) (*usecasemodels.Order, error) {
	if !validStatus(newStatus) {
		return nil, order.ErrInvalidStatus
	}
	o, _, err := uc.repo.FindOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	current := usecasemodels.OrderStatus(o.Status)
	if !canTransition(current, newStatus) {
		return nil, order.ErrInvalidStatusTransition
	}
	updated, err := uc.repo.UpdateOrderStatus(ctx, orderID, string(newStatus))
	if err != nil {
		return nil, err
	}
	// Подгрузим позиции для возврата полноценного Order
	_, items, err := uc.repo.FindOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return uc.assemble(updated, items), nil
}

// validStatus проверяет, что newStatus принадлежит enum'у
func validStatus(s usecasemodels.OrderStatus) bool {
	switch s {
	case usecasemodels.OrderStatusAccepted,
		usecasemodels.OrderStatusCooking,
		usecasemodels.OrderStatusReady,
		usecasemodels.OrderStatusInDelivery,
		usecasemodels.OrderStatusClosed,
		usecasemodels.OrderStatusCancelled:
		return true
	}
	return false
}

// canTransition проверяет, что переход from → to разрешён state-machine
func canTransition(from, to usecasemodels.OrderStatus) bool {
	allowed, ok := allowedStatusTransitions[from]
	if !ok {
		// from — терминальный (closed/cancelled), переходов нет
		return false
	}
	_, found := allowed[to]
	return found
}
