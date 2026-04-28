package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
)

const sessionKeyPrefix = "session:"

// Load возвращает сессию и обновляет TTL
func (r *Repository) Load(ctx context.Context, id uuid.UUID) (*repositorymodels.Session, error) {
	raw, err := r.redis.Get(ctx, sessionKey(id))
	if errors.Is(err, goredis.Nil) {
		return nil, session.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var s repositorymodels.Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if err := r.redis.Expire(ctx, sessionKey(id), r.ttl); err != nil {
		return nil, fmt.Errorf("redis expire: %w", err)
	}
	return &s, nil
}

// Save сохраняет сессию с TTL
func (r *Repository) Save(ctx context.Context, id uuid.UUID, s *repositorymodels.Session) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := r.redis.Set(ctx, sessionKey(id), string(raw), r.ttl); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// Delete удаляет сессию
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.redis.Del(ctx, sessionKey(id)); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

func sessionKey(id uuid.UUID) string { return sessionKeyPrefix + id.String() }
