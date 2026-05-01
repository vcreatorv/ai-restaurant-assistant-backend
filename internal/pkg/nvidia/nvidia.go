// Package nvidia — фабрика llm.Client'а для провайдера NVIDIA NIM (build.nvidia.com).
// API OpenAI-совместимый, поэтому вся логика живёт в internal/pkg/llm; здесь
// только разводка конфига (base_url, api_key, model) в llm.Config. В отличие от
// OpenRouter, NVIDIA не требует HTTP-Referer / X-Title.
package nvidia

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

// New собирает llm.Client с настройками NVIDIA NIM.
// providerCfg — параметры конкретно NVIDIA (base_url, api_key, model),
// commonCfg — общие параметры запроса (temperature, max_tokens, timeouts).
func New(providerCfg rag.NvidiaConfig, commonCfg rag.LLMCommonConfig) (*llm.Client, error) {
	return llm.New(llm.Config{
		Provider:          "nvidia",
		BaseURL:           providerCfg.BaseURL,
		APIKey:            providerCfg.APIKey,
		Model:             providerCfg.Model,
		Temperature:       commonCfg.Temperature,
		MaxTokens:         commonCfg.MaxTokens,
		RequestTimeout:    commonCfg.RequestTimeout,
		FirstTokenTimeout: commonCfg.FirstTokenTimeout,
	})
}
