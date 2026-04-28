package app

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// DefaultConfigPath путь к конфигу по умолчанию
const DefaultConfigPath = "configs/config.yaml"

// Config конфигурация приложения
type Config struct {
	// HTTP параметры HTTP-сервера и cookie
	HTTP HTTPConfig `yaml:"http"`
	// Log параметры логгера
	Log LogConfig `yaml:"log"`
	// Postgres параметры PostgreSQL
	Postgres datasources.PostgresConfig `yaml:"postgres"`
	// Redis параметры Redis
	Redis datasources.RedisConfig `yaml:"redis"`
	// Session параметры фичи session
	Session session.Config `yaml:"session"`
	// Auth параметры фичи auth
	Auth auth.Config `yaml:"auth"`
	// User параметры фичи user
	User user.Config `yaml:"user"`
}

// HTTPConfig параметры HTTP-сервера и cookie
type HTTPConfig struct {
	// Addr адрес для прослушивания
	Addr string `yaml:"addr"`
	// CookieSecure флаг Secure для cookie
	CookieSecure bool `yaml:"cookie_secure"`
}

// LogConfig параметры логгера
type LogConfig struct {
	// Level уровень логирования
	Level slog.Level `yaml:"level"`
}

// LoadConfig читает yaml-конфиг из path и оверрайдит секреты из env
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		path = v
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		c.Postgres.DSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.Redis.Password = v
	}

	if c.Postgres.DSN == "" {
		return nil, fmt.Errorf("postgres.dsn is required (set POSTGRES_DSN env)")
	}
	if c.HTTP.Addr == "" {
		return nil, fmt.Errorf("http.addr is required")
	}
	if c.Session.Repository.TTL <= 0 {
		return nil, fmt.Errorf("session.repository.ttl is required")
	}
	if c.Auth.Usecase.BcryptCost <= 0 {
		return nil, fmt.Errorf("auth.usecase.bcrypt_cost is required")
	}

	return c, nil
}
