// Package openrouter — фабрика llm.Client'а для провайдера OpenRouter.
// Сама HTTP/SSE-логика живёт в internal/pkg/llm; здесь только разводка
// конфига (base_url, api_key, model, referer/title) в llm.Config.
package openrouter

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

// New собирает llm.Client с настройками OpenRouter.
// providerCfg — параметры конкретно OpenRouter (base_url, api_key, model, headers),
// commonCfg — общие параметры запроса (temperature, max_tokens, timeouts).
func New(providerCfg rag.OpenRouterConfig, commonCfg rag.LLMCommonConfig) (*llm.Client, error) {
	return llm.New(llm.Config{
		Provider:          "openrouter",
		BaseURL:           providerCfg.BaseURL,
		APIKey:            providerCfg.APIKey,
		Model:             providerCfg.Model,
		Temperature:       commonCfg.Temperature,
		MaxTokens:         commonCfg.MaxTokens,
		RequestTimeout:    commonCfg.RequestTimeout,
		FirstTokenTimeout: commonCfg.FirstTokenTimeout,
		ExtraHeaders: map[string]string{
			"HTTP-Referer": providerCfg.Referer,
			"X-Title":      providerCfg.Title,
		},
	})
}
