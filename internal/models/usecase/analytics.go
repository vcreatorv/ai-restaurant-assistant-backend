package usecase

// OrdersByStatusEntry разбивка количества заказов по статусам.
type OrdersByStatusEntry struct {
	Status string
	Count  int
}

// TopDishEntry строка топа: блюдо + значение метрики (заказано / рекомендовано).
type TopDishEntry struct {
	DishID   int
	DishName string
	Value    int
}

// AdminDashboard операционные метрики за период.
type AdminDashboard struct {
	Period                  string
	Orders                  int
	RevenueMinor            int64
	AverageCheckMinor       int64
	OrdersByStatus          []OrdersByStatusEntry
	TopDishes               []TopDishEntry
	// CartAdditionsTotal — общее число «+» в корзину за период (все источники).
	CartAdditionsTotal int
	// CartAdditionsFromChat — добавления из карточки в ответе ассистента (source='chat').
	CartAdditionsFromChat int
	// CartAdditionsFromMenu — добавления со страницы публичного меню (source='menu').
	CartAdditionsFromMenu int
}

// AdminAnalytics поведенческие метрики ассистента за период.
type AdminAnalytics struct {
	Period                    string
	AssistantMessages         int
	AvgOrderedPerMessage      float64
	AvgAddedToCartPerMessage  float64
	TopRecommendedDishes      []TopDishEntry
}
