package redis

import (
	"time"

	pkgredis "github.com/example/ai-restaurant-assistant-backend/internal/pkg/redis"
)

// Repository Redis-репозиторий сессий
type Repository struct {
	redis pkgredis.Redis
	ttl   time.Duration
}

// New создаёт Repository
func New(redis pkgredis.Redis, ttl time.Duration) *Repository {
	return &Repository{redis: redis, ttl: ttl}
}
