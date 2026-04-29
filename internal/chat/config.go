package chat

import "time"

// Config параметры фичи chat
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

// UsecaseConfig параметры chat usecase
type UsecaseConfig struct {
	// AutoNewChatAfter порог давности активного чата; если последний чат старше — создаём новый
	AutoNewChatAfter time.Duration `yaml:"auto_new_chat_after"`
}

// DeliveryConfig параметры HTTP-делирия
type DeliveryConfig struct {
	// MessagesDefaultLimit limit по умолчанию для history-эндпойнта
	MessagesDefaultLimit int `yaml:"messages_default_limit"`
	// MessagesMaxLimit верхний предел limit
	MessagesMaxLimit int `yaml:"messages_max_limit"`
	// ListDefaultLimit limit по умолчанию для списка чатов
	ListDefaultLimit int `yaml:"list_default_limit"`
	// ListMaxLimit верхний предел limit для списка чатов
	ListMaxLimit int `yaml:"list_max_limit"`
}
