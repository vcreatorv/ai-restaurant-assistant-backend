package auth

// Config параметры фичи auth
type Config struct {
	// Usecase конфиг бизнес-логики
	Usecase UsecaseConfig `yaml:"usecase"`
	// Delivery конфиг HTTP-делирия
	Delivery DeliveryConfig `yaml:"delivery"`
}

// UsecaseConfig параметры auth usecase
type UsecaseConfig struct {
	// BcryptCost стоимость bcrypt для хэширования пароля
	BcryptCost int `yaml:"bcrypt_cost"`
}

// DeliveryConfig параметры HTTP-делирия
type DeliveryConfig struct{}
