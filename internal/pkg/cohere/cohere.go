// Package cohere — HTTP-клиент Cohere API (Embed v2, Rerank v2).
package cohere

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

// ErrEmptyAPIKey не задан Cohere API key
var ErrEmptyAPIKey = errors.New("cohere: empty api key")

// Client клиент Cohere API
type Client struct {
	cfg  rag.CohereConfig
	http *http.Client
}

// New создаёт клиент Cohere
func New(cfg rag.CohereConfig) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrEmptyAPIKey
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.RequestTimeout},
	}, nil
}

// embedRequest тело запроса Embed v2
type embedRequest struct {
	Model          string   `json:"model"`
	Texts          []string `json:"texts"`
	InputType      string   `json:"input_type"`
	EmbeddingTypes []string `json:"embedding_types"`
	Truncate       string   `json:"truncate,omitempty"`
}

// embedResponse фрагмент ответа Embed v2
type embedResponse struct {
	Embeddings struct {
		Float [][]float32 `json:"float"`
	} `json:"embeddings"`
}

// rerankRequest тело запроса Rerank v2
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResponse фрагмент ответа Rerank v2
type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// RerankResult результат рерэнкинга одного документа
type RerankResult struct {
	// Index исходный индекс документа в массиве documents
	Index int
	// Score релевантность 0..1; чем выше — тем лучше
	Score float64
}

// Rerank переранжирует documents относительно query, возвращает топ-N результатов
func (c *Client) Rerank(
	ctx context.Context,
	query string,
	documents []string,
	topN int,
) ([]RerankResult, error) {
	if len(documents) == 0 {
		return []RerankResult{}, nil
	}
	log := logger.ForCtx(ctx).With("service", "cohere", "op", "rerank")
	body := rerankRequest{
		Model:     c.cfg.RerankModel,
		Query:     query,
		Documents: documents,
		TopN:      topN,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal rerank body: %w", err)
	}

	start := time.Now()
	resp, err := c.doWithRetry(ctx, http.MethodPost, c.cfg.BaseURL+"/v2/rerank", raw, "rerank")
	if err != nil {
		log.Error("cohere rerank failed",
			"err", err,
			"docs", len(documents),
			"top_n", topN,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Error("cohere rerank non-2xx",
			"status", resp.StatusCode,
			"body", string(errBody),
			"docs", len(documents),
			"top_n", topN,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, fmt.Errorf("cohere rerank: status %d: %s", resp.StatusCode, string(errBody))
	}

	var parsed rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode rerank response: %w", err)
	}
	out := make([]RerankResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, RerankResult{Index: r.Index, Score: r.RelevanceScore})
	}
	log.Info("cohere rerank ok",
		"status", resp.StatusCode,
		"docs", len(documents),
		"top_n", topN,
		"results", len(out),
		"top_score", topRerankScore(out),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return out, nil
}

// topRerankScore возвращает score первого результата (0 если пусто)
func topRerankScore(rs []RerankResult) float64 {
	if len(rs) == 0 {
		return 0
	}
	return rs[0].Score
}

// Embed эмбеддит batch текстов в режиме inputType (search_document / search_query)
func (c *Client) Embed(
	ctx context.Context,
	texts []string,
	inputType rag.CohereInputType,
) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if len(texts) > c.cfg.EmbedBatchSize {
		return nil, fmt.Errorf("cohere embed: batch size %d exceeds limit %d",
			len(texts), c.cfg.EmbedBatchSize)
	}
	log := logger.ForCtx(ctx).With("service", "cohere", "op", "embed", "input_type", string(inputType))

	body := embedRequest{
		Model:          c.cfg.EmbedModel,
		Texts:          texts,
		InputType:      string(inputType),
		EmbeddingTypes: []string{"float"},
		Truncate:       "END",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embed body: %w", err)
	}

	start := time.Now()
	resp, err := c.doWithRetry(ctx, http.MethodPost, c.cfg.BaseURL+"/v2/embed", raw, "embed")
	if err != nil {
		log.Error("cohere embed failed",
			"err", err,
			"batch", len(texts),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Error("cohere embed non-2xx",
			"status", resp.StatusCode,
			"body", string(errBody),
			"batch", len(texts),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, fmt.Errorf("cohere embed: status %d: %s", resp.StatusCode, string(errBody))
	}

	var parsed embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(parsed.Embeddings.Float) != len(texts) {
		return nil, fmt.Errorf("cohere embed: expected %d vectors, got %d",
			len(texts), len(parsed.Embeddings.Float))
	}
	dim := 0
	if len(parsed.Embeddings.Float) > 0 {
		dim = len(parsed.Embeddings.Float[0])
	}
	log.Info("cohere embed ok",
		"status", resp.StatusCode,
		"batch", len(texts),
		"dim", dim,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return parsed.Embeddings.Float, nil
}

// doWithRetry шлёт HTTP-запрос с повторами на сетевые ошибки (TLS handshake timeout, dial timeout).
// op — короткое имя операции для логов ("embed" / "rerank").
func (c *Client) doWithRetry(ctx context.Context, method, url string, body []byte, op string) (*http.Response, error) {
	log := logger.ForCtx(ctx).With("service", "cohere", "op", op, "method", method, "url", url)
	attempts := c.cfg.RetryAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		attempt := i + 1
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")

		t := time.Now()
		resp, err := c.http.Do(req)
		dur := time.Since(t).Milliseconds()
		if err == nil {
			log.Debug("cohere http call",
				"attempt", attempt,
				"max_attempts", attempts,
				"status", resp.StatusCode,
				"duration_ms", dur,
			)
			return resp, nil
		}
		lastErr = err
		retryable := isRetryable(err)
		remaining := attempts - attempt
		if !retryable || remaining <= 0 {
			log.Error("cohere http call failed",
				"attempt", attempt,
				"max_attempts", attempts,
				"retryable", retryable,
				"err", err,
				"duration_ms", dur,
			)
			if !retryable {
				return nil, err
			}
			break
		}
		log.Warn("cohere http retry",
			"attempt", attempt,
			"max_attempts", attempts,
			"remaining", remaining,
			"err", err,
			"duration_ms", dur,
			"retry_delay_ms", c.cfg.RetryDelay.Milliseconds(),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.cfg.RetryDelay):
		}
	}
	return nil, lastErr
}

// isRetryable распознаёт сетевые ошибки, которые имеет смысл повторить (timeouts, temporary)
func isRetryable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
