package order

// Config параметры фичи «заказы»
type Config struct {
	// Repository конфиг репозитория
	Repository RepositoryConfig `yaml:"repository"`
	// Usecase конфиг бизнес-логики
	Usecase UsecaseConfig `yaml:"usecase"`
	// Delivery конфиг HTTP-делирия
	Delivery DeliveryConfig `yaml:"delivery"`
}

// RepositoryConfig параметры postgres-репо
type RepositoryConfig struct{}

// UsecaseConfig параметры order usecase
type UsecaseConfig struct{}

// DeliveryConfig параметры HTTP-делирия
type DeliveryConfig struct {
	// ListDefaultLimit limit по умолчанию для GET /orders
	ListDefaultLimit int `yaml:"list_default_limit"`
	// ListMaxLimit верхний предел
	ListMaxLimit int `yaml:"list_max_limit"`
}
