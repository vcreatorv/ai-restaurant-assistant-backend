package menu

// Config параметры фичи menu
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

// UsecaseConfig параметры menu usecase
type UsecaseConfig struct{}

// DeliveryConfig параметры HTTP-делирия
type DeliveryConfig struct {
	// DefaultLimit limit по умолчанию для list-эндпойнтов
	DefaultLimit int `yaml:"default_limit"`
	// MaxLimit верхний предел limit
	MaxLimit int `yaml:"max_limit"`
	// MaxImageSizeBytes максимальный размер загружаемой картинки
	MaxImageSizeBytes int64 `yaml:"max_image_size_bytes"`
}
