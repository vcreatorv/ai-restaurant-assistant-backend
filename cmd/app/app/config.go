package app

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
	"github.com/example/ai-restaurant-assistant-backend/internal/session"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"gopkg.in/yaml.v3"
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
	// S3 параметры S3-совместимого хранилища
	S3 s3.Config `yaml:"s3"`
	// Session параметры фичи session
	Session session.Config `yaml:"session"`
	// Auth параметры фичи auth
	Auth auth.Config `yaml:"auth"`
	// User параметры фичи user
	User user.Config `yaml:"user"`
	// Menu параметры фичи menu
	Menu menu.Config `yaml:"menu"`
	// Chat параметры фичи chat
	Chat chat.Config `yaml:"chat"`
	// RAG параметры RAG-pipeline (Cohere + Qdrant)
	RAG rag.Config `yaml:"rag"`
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
	if v := os.Getenv("S3_ENDPOINT"); v != "" {
		c.S3.Endpoint = v
	}
	if v := os.Getenv("S3_ACCESS_KEY"); v != "" {
		c.S3.AccessKey = v
	}
	if v := os.Getenv("S3_SECRET_KEY"); v != "" {
		c.S3.SecretKey = v
	}
	if v := os.Getenv("S3_BUCKET"); v != "" {
		c.S3.Bucket = v
	}
	if v := os.Getenv("S3_PUBLIC_BASE_URL"); v != "" {
		c.S3.PublicBaseURL = v
	}
	if v := os.Getenv("COHERE_API_KEY"); v != "" {
		c.RAG.Cohere.APIKey = v
	}
	if v := os.Getenv("QDRANT_URL"); v != "" {
		c.RAG.Qdrant.URL = v
	}
	if v := os.Getenv("QDRANT_API_KEY"); v != "" {
		c.RAG.Qdrant.APIKey = v
	}
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		c.RAG.LLM.OpenRouter.APIKey = v
	}
	if v := os.Getenv("NVIDIA_API_KEY"); v != "" {
		c.RAG.LLM.Nvidia.APIKey = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		c.RAG.LLM.Provider = v
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
	if c.Menu.Delivery.DefaultLimit <= 0 {
		c.Menu.Delivery.DefaultLimit = 20
	}
	if c.Menu.Delivery.MaxLimit <= 0 {
		c.Menu.Delivery.MaxLimit = 100
	}
	if c.Menu.Delivery.MaxImageSizeBytes <= 0 {
		c.Menu.Delivery.MaxImageSizeBytes = 5 * 1024 * 1024
	}

	if c.Chat.Usecase.AutoNewChatAfter <= 0 {
		c.Chat.Usecase.AutoNewChatAfter = 6 * time.Hour
	}
	if c.Chat.Delivery.MessagesDefaultLimit <= 0 {
		c.Chat.Delivery.MessagesDefaultLimit = 50
	}
	if c.Chat.Delivery.MessagesMaxLimit <= 0 {
		c.Chat.Delivery.MessagesMaxLimit = 200
	}
	if c.Chat.Delivery.ListDefaultLimit <= 0 {
		c.Chat.Delivery.ListDefaultLimit = 20
	}
	if c.Chat.Delivery.ListMaxLimit <= 0 {
		c.Chat.Delivery.ListMaxLimit = 100
	}

	if c.S3.Endpoint == "" {
		return nil, fmt.Errorf("s3.endpoint is required")
	}
	if c.S3.Bucket == "" {
		return nil, fmt.Errorf("s3.bucket is required")
	}

	return c, nil
}
