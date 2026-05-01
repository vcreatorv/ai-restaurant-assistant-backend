// Package llm — общий HTTP-клиент chat-completions (OpenAI-совместимый протокол).
// Используется обоими провайдерами LLM, между которыми переключается приложение:
// OpenRouter и NVIDIA NIM (build.nvidia.com). Различия — base_url, api_key и
// набор дополнительных headers — задаются через Config.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
)

// ErrEmptyAPIKey не задан api key
var ErrEmptyAPIKey = errors.New("llm: empty api key")

// ErrFirstTokenTimeout первый токен не пришёл в отведённое время
var ErrFirstTokenTimeout = errors.New("llm: first token timeout")

// ErrUpstreamFailure ошибка выше по стеку (провайдер LLM)
var ErrUpstreamFailure = errors.New("llm: upstream failure")

// Role роль сообщения чата
type Role string

const (
	// RoleSystem системное сообщение
	RoleSystem Role = "system"
	// RoleUser сообщение пользователя
	RoleUser Role = "user"
	// RoleAssistant сообщение ассистента
	RoleAssistant Role = "assistant"
)

// Message сообщение чата для chat-completions
type Message struct {
	// Role роль автора (system, user, assistant)
	Role Role `json:"role"`
	// Content текст сообщения
	Content string `json:"content"`
}

// ChatRequest параметры одного запроса chat-completions
type ChatRequest struct {
	// Messages массив сообщений (system + history + user)
	Messages []Message
	// ModelOverride если задан, переопределяет cfg.Model
	ModelOverride string
}

// Usage агрегированная телеметрия одного ответа
type Usage struct {
	// PromptTokens сколько токенов входа учла модель
	PromptTokens int
	// CompletionTokens сколько токенов выхода сгенерировано
	CompletionTokens int
	// FinishReason причина завершения (stop, length, content_filter, ...)
	FinishReason string
	// Model фактическая модель, которой ответил провайдер
	Model string
}

// Config параметры одного экземпляра клиента (один провайдер)
type Config struct {
	// Provider короткое имя провайдера для логов ("openrouter", "nvidia")
	Provider string
	// BaseURL базовый URL API (без хвостового слэша, без /chat/completions)
	BaseURL string
	// APIKey ключ API (Bearer)
	APIKey string
	// Model имя модели по умолчанию (можно переопределить через ChatRequest.ModelOverride)
	Model string
	// Temperature 0..1
	Temperature float64
	// MaxTokens максимум выходных токенов
	MaxTokens int
	// RequestTimeout таймаут на весь HTTP-запрос (включая стрим)
	RequestTimeout time.Duration
	// FirstTokenTimeout таймаут до получения первого токена в SSE-стриме
	FirstTokenTimeout time.Duration
	// ExtraHeaders дополнительные headers (для OpenRouter — HTTP-Referer / X-Title)
	ExtraHeaders map[string]string
}

// Client клиент chat-completions
type Client struct {
	cfg  Config
	http *http.Client
}

// New создаёт клиент. Возвращает ErrEmptyAPIKey, если в cfg не задан APIKey.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrEmptyAPIKey
	}
	if cfg.Provider == "" {
		cfg.Provider = "llm"
	}
	// Кастомный transport: дефолтный TLSHandshakeTimeout=10s слишком жёсткий
	// для походов из РФ; держим TLS handshake до 30s, остальное — по дефолтам.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.RequestTimeout, Transport: transport},
	}, nil
}

// Provider возвращает короткое имя провайдера (для логирования)
func (c *Client) Provider() string {
	return c.cfg.Provider
}

// chatBody тело HTTP-запроса chat-completions
type chatBody struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// chatChunk SSE-фрейм chat-completions
type chatChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ChatStream шлёт chat-completions с stream=true и зовёт onDelta на каждый токен
func (c *Client) ChatStream(
	ctx context.Context,
	req ChatRequest,
	onDelta func(string) error,
) (Usage, error) {
	model := c.cfg.Model
	if req.ModelOverride != "" {
		model = req.ModelOverride
	}
	log := logger.ForCtx(ctx).With(
		"service", c.cfg.Provider,
		"op", "chat-completions",
		"model_requested", model,
		"messages", len(req.Messages),
	)
	body := chatBody{
		Model:       model,
		Messages:    req.Messages,
		Stream:      true,
		Temperature: c.cfg.Temperature,
		MaxTokens:   c.cfg.MaxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Usage{}, fmt.Errorf("marshal chat body: %w", err)
	}

	start := time.Now()
	resp, err := c.doWithRetry(ctx, raw)
	if err != nil {
		return Usage{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Error(c.cfg.Provider+" chat non-2xx",
			"status", resp.StatusCode,
			"body", strings.TrimSpace(string(errBody)),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return Usage{}, fmt.Errorf("%w: %s status %d: %s",
			ErrUpstreamFailure, c.cfg.Provider, resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	usage, ttftMS, err := c.consumeStream(ctx, resp.Body, onDelta)
	totalMS := time.Since(start).Milliseconds()
	if err != nil {
		log.Error(c.cfg.Provider+" chat stream failed",
			"err", err,
			"ttft_ms", ttftMS,
			"duration_ms", totalMS,
			"model_actual", usage.Model,
			"tokens_in", usage.PromptTokens,
			"tokens_out", usage.CompletionTokens,
		)
		return usage, err
	}
	log.Info(c.cfg.Provider+" chat ok",
		"status", resp.StatusCode,
		"ttft_ms", ttftMS,
		"duration_ms", totalMS,
		"model_actual", usage.Model,
		"tokens_in", usage.PromptTokens,
		"tokens_out", usage.CompletionTokens,
		"finish_reason", usage.FinishReason,
	)
	return usage, nil
}

// doWithRetry шлёт HTTP-запрос chat-completions с повторами на сетевые таймауты.
// Безопасно повторять только до того, как мы начали потреблять SSE-стрим.
func (c *Client) doWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	url := c.cfg.BaseURL + "/chat/completions"
	log := logger.ForCtx(ctx).With(
		"service", c.cfg.Provider,
		"op", "chat-completions",
		"method", http.MethodPost,
		"url", url,
	)
	const maxAttempts = 2
	const retryDelay = 500 * time.Millisecond
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		attempt := i + 1
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		for k, v := range c.cfg.ExtraHeaders {
			if v == "" {
				continue
			}
			req.Header.Set(k, v)
		}

		t := time.Now()
		resp, err := c.http.Do(req)
		dur := time.Since(t).Milliseconds()
		if err == nil {
			log.Debug(c.cfg.Provider+" http call",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"status", resp.StatusCode,
				"duration_ms", dur,
			)
			return resp, nil
		}
		lastErr = err
		retryable := isRetryable(err)
		remaining := maxAttempts - attempt
		if !retryable || remaining <= 0 {
			log.Error(c.cfg.Provider+" http call failed",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"retryable", retryable,
				"err", err,
				"duration_ms", dur,
			)
			if !retryable {
				return nil, err
			}
			break
		}
		log.Warn(c.cfg.Provider+" http retry",
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"remaining", remaining,
			"err", err,
			"duration_ms", dur,
			"retry_delay_ms", retryDelay.Milliseconds(),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return nil, lastErr
}

// isRetryable распознаёт сетевые ошибки, которые имеет смысл повторить (timeouts)
func isRetryable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// consumeStream читает SSE-стрим, парсит chat-completions chunk-и и зовёт onDelta.
// Возвращает Usage, ttftMS (время до первого токена в мс; 0 если не дошёл) и ошибку.
func (c *Client) consumeStream(
	ctx context.Context,
	body io.Reader,
	onDelta func(string) error,
) (Usage, int64, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	firstToken := time.NewTimer(c.cfg.FirstTokenTimeout)
	defer firstToken.Stop()

	streamStart := time.Now()
	var ttftMS int64
	got := false
	usage := Usage{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			// keepalive comment
			if !got {
				if err := checkFirstToken(ctx, firstToken); err != nil {
					return usage, ttftMS, err
				}
			}
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// невалидный JSON в стриме игнорируем — это могут быть keepalive-фрагменты
			continue
		}
		if chunk.Model != "" {
			usage.Model = chunk.Model
		}
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		if ch.FinishReason != "" {
			usage.FinishReason = ch.FinishReason
		}
		if ch.Delta.Content == "" {
			continue
		}
		if !got {
			ttftMS = time.Since(streamStart).Milliseconds()
		}
		got = true
		firstToken.Stop()
		if err := onDelta(ch.Delta.Content); err != nil {
			return usage, ttftMS, err
		}
	}
	if err := scanner.Err(); err != nil {
		return usage, ttftMS, fmt.Errorf("read stream: %w", err)
	}
	if !got {
		return usage, ttftMS, ErrFirstTokenTimeout
	}
	return usage, ttftMS, nil
}

// checkFirstToken отдаёт ErrFirstTokenTimeout, если first_token_timeout уже истёк
func checkFirstToken(ctx context.Context, t *time.Timer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return ErrFirstTokenTimeout
	default:
		return nil
	}
}
