package session

import "time"

// Config параметры фичи session
type Config struct {
	// Repository конфиг репозитория
	Repository RepositoryConfig `yaml:"repository"`
	// Usecase конфиг бизнес-логики
	Usecase UsecaseConfig `yaml:"usecase"`
}

// RepositoryConfig параметры Redis-репозитория сессий
type RepositoryConfig struct {
	// TTL время жизни сессии (sliding)
	TTL time.Duration `yaml:"ttl"`
}

// UsecaseConfig параметры session usecase
type UsecaseConfig struct{}
