package cart

// Config параметры фичи «корзина»
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

// UsecaseConfig параметры cart usecase
type UsecaseConfig struct{}

// DeliveryConfig параметры HTTP-делирия
type DeliveryConfig struct{}
