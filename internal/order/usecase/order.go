package usecase

import (
	"context"
	"fmt"
	"strings"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/order"

	"github.com/google/uuid"
)

// currencyDefault код валюты на MVP — единственный для всех заказов
const currencyDefault = "RUB"

// Create оформляет заказ из текущей корзины пользователя и чистит её.
// Шаги:
//  1. валидация fulfillment_type / payment_method (поверх OpenAPI-валидатора, страховка);
//  2. проверка профиля (first_name + last_name + phone);
//  3. для delivery — обязателен delivery_address;
//  4. чтение корзины + проверка is_available по каждой позиции;
//  5. snapshot цен/имён в order_items, расчёт total_minor;
//  6. транзакционная вставка orders + order_items;
//  7. очистка корзины.
func (uc *orderUsecase) Create(
	ctx context.Context,
	userID uuid.UUID,
	req usecasemodels.CreateOrderRequest,
) (*usecasemodels.Order, error) {
	if !validFulfillment(req.FulfillmentType) {
		return nil, order.ErrInvalidFulfillmentType
	}
	if !validPayment(req.PaymentMethod) {
		return nil, order.ErrInvalidPaymentMethod
	}

	profile, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	if profile == nil ||
		strings.TrimSpace(profile.FirstName) == "" ||
		strings.TrimSpace(profile.LastName) == "" ||
		strings.TrimSpace(profile.Phone) == "" {
		return nil, order.ErrProfileIncomplete
	}

	if req.FulfillmentType == usecasemodels.FulfillmentDelivery {
		if req.DeliveryAddress == nil || strings.TrimSpace(*req.DeliveryAddress) == "" {
			return nil, order.ErrDeliveryAddressRequired
		}
	}

	cartRow, err := uc.cartRepo.FindOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find cart: %w", err)
	}
	cartItems, err := uc.cartRepo.ListItems(ctx, cartRow.ID)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}
	if len(cartItems) == 0 {
		return nil, order.ErrCartEmpty
	}

	// Подгружаем актуальные данные блюд: name, price, is_available.
	dishIDs := make([]int, 0, len(cartItems))
	for _, it := range cartItems {
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

	unavailable := make([]int, 0)
	for _, it := range cartItems {
		d, ok := dishesByID[it.DishID]
		if !ok || !d.IsAvailable {
			unavailable = append(unavailable, it.DishID)
		}
	}
	if len(unavailable) > 0 {
		return nil, &order.CheckoutItemsUnavailableError{DishIDs: unavailable}
	}

	// Snapshot позиций + total
	orderItems := make([]repositorymodels.OrderItem, 0, len(cartItems))
	totalMinor := 0
	for _, it := range cartItems {
		d := dishesByID[it.DishID]
		line := d.PriceMinor * it.Quantity
		totalMinor += line
		orderItems = append(orderItems, repositorymodels.OrderItem{
			DishID:             it.DishID,
			NameSnapshot:       d.Name,
			PriceMinorSnapshot: d.PriceMinor,
			Quantity:           it.Quantity,
			SortOrder:          it.SortOrder,
		})
	}

	o := &repositorymodels.Order{
		ID:                uc.uuid.New(),
		UserID:            userID,
		Status:            string(usecasemodels.OrderStatusAccepted),
		FulfillmentType:   string(req.FulfillmentType),
		PaymentMethod:     string(req.PaymentMethod),
		TotalMinor:        totalMinor,
		Currency:          currencyDefault,
		CustomerFirstName: profile.FirstName,
		CustomerLastName:  profile.LastName,
		CustomerPhone:     profile.Phone,
		CustomerEmail:     stringPtrIfNotEmpty(profile.Email),
		DeliveryAddress:   trimmedPtr(req.DeliveryAddress),
		DeliveryNotes:     trimmedPtr(req.DeliveryNotes),
		Notes:             trimmedPtr(req.Notes),
	}
	if req.FulfillmentType != usecasemodels.FulfillmentDelivery {
		// Для не-delivery — игнорируем delivery-поля даже если пришли.
		o.DeliveryAddress = nil
		o.DeliveryNotes = nil
	}

	if err := uc.repo.CreateOrder(ctx, o, orderItems); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	// Чистим корзину после успешного оформления. Если очистка упала — заказ всё
	// равно создан; пользователь увидит «остатки» в корзине и сможет очистить вручную.
	if err := uc.cartRepo.DeleteAllItems(ctx, cartRow.ID); err != nil {
		// Логировать здесь негде (нет logger в сигнатуре usecase) — пробрасываем
		// в виде wrapped error, чтобы responseErrorHandler залогировал. Frontend
		// получит 500, но заказ при этом виден в /orders.
		return nil, fmt.Errorf("clear cart after checkout: %w", err)
	}

	return uc.assemble(o, orderItems), nil
}

// List возвращает заказы пользователя с фильтром и пагинацией
func (uc *orderUsecase) List(
	ctx context.Context,
	userID uuid.UUID,
	filter usecasemodels.OrderListFilter,
) ([]usecasemodels.Order, int, error) {
	repoFilter := repositorymodels.OrderFilter{
		UserID: &userID,
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

// Get возвращает заказ по id; проверяет владельца
func (uc *orderUsecase) Get(
	ctx context.Context,
	userID, orderID uuid.UUID,
) (*usecasemodels.Order, error) {
	o, items, err := uc.repo.FindOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if o.UserID != userID {
		return nil, order.ErrOrderForbidden
	}
	return uc.assemble(o, items), nil
}

// assemble собирает usecase-модель Order из repo-моделей
func (uc *orderUsecase) assemble(
	o *repositorymodels.Order,
	items []repositorymodels.OrderItem,
) *usecasemodels.Order {
	out := &usecasemodels.Order{
		ID:                o.ID,
		UserID:            o.UserID,
		Status:            usecasemodels.OrderStatus(o.Status),
		FulfillmentType:   usecasemodels.FulfillmentType(o.FulfillmentType),
		PaymentMethod:     usecasemodels.PaymentMethod(o.PaymentMethod),
		TotalMinor:        o.TotalMinor,
		Currency:          o.Currency,
		CustomerFirstName: o.CustomerFirstName,
		CustomerLastName:  o.CustomerLastName,
		CustomerPhone:     o.CustomerPhone,
		CustomerEmail:     o.CustomerEmail,
		DeliveryAddress:   o.DeliveryAddress,
		DeliveryNotes:     o.DeliveryNotes,
		Notes:             o.Notes,
		Items:             make([]usecasemodels.OrderItem, 0, len(items)),
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}
	for i := range items {
		it := &items[i]
		dishID := it.DishID
		out.Items = append(out.Items, usecasemodels.OrderItem{
			DishID:         &dishID,
			DishName:       it.NameSnapshot,
			DishPriceMinor: it.PriceMinorSnapshot,
			Quantity:       it.Quantity,
			LineTotalMinor: it.PriceMinorSnapshot * it.Quantity,
			SortOrder:      it.SortOrder,
		})
	}
	return out
}

// validFulfillment проверяет fulfillment_type на принадлежность enum'у
func validFulfillment(t usecasemodels.FulfillmentType) bool {
	switch t {
	case usecasemodels.FulfillmentDelivery,
		usecasemodels.FulfillmentPickup,
		usecasemodels.FulfillmentDineIn:
		return true
	}
	return false
}

// validPayment проверяет payment_method на принадлежность enum'у
func validPayment(p usecasemodels.PaymentMethod) bool {
	switch p {
	case usecasemodels.PaymentOnDelivery, usecasemodels.PaymentOnlineStub:
		return true
	}
	return false
}

// trimmedPtr триммит и возвращает указатель на строку; nil если пустая после trim
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// stringPtrIfNotEmpty возвращает указатель на s или nil если пустая
func stringPtrIfNotEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
