// Package qdrant — HTTP-клиент Qdrant (collections, payload-индексы, upsert points).
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
)

const (
	// FieldTypeKeyword индекс по строковому payload-полю
	FieldTypeKeyword = "keyword"
	// FieldTypeInteger индекс по числовому payload-полю
	FieldTypeInteger = "integer"
	// FieldTypeBool индекс по булевому payload-полю
	FieldTypeBool = "bool"
	// FieldTypeFloat индекс по float payload-полю
	FieldTypeFloat = "float"
)

// Client клиент Qdrant
type Client struct {
	cfg  rag.QdrantConfig
	http *http.Client
}

// New создаёт клиент Qdrant
func New(cfg rag.QdrantConfig) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.RequestTimeout},
	}
}

// Point единица данных Qdrant: id + вектор + payload
type Point struct {
	// ID идентификатор точки (uint64); используем dish_id
	ID uint64
	// Vector вектор-эмбеддинг
	Vector []float32
	// Payload произвольный JSON с метаданными
	Payload map[string]any
}

// PayloadIndex описывает индекс по payload-полю
type PayloadIndex struct {
	// Field имя поля в payload
	Field string
	// Type тип индекса (keyword | integer | bool | float)
	Type string
}

// EnsureCollection создаёт коллекцию идемпотентно
func (c *Client) EnsureCollection(ctx context.Context, vectorSize int) error {
	exists, err := c.collectionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": c.cfg.Distance,
		},
		"hnsw_config": map[string]any{
			"m":            c.cfg.HNSW.M,
			"ef_construct": c.cfg.HNSW.EfConstruct,
		},
	}
	return c.put(ctx, "/collections/"+c.cfg.Collection, body, nil)
}

// EnsurePayloadIndexes создаёт payload-индексы идемпотентно
func (c *Client) EnsurePayloadIndexes(ctx context.Context, indexes []PayloadIndex) error {
	for _, idx := range indexes {
		body := map[string]any{
			"field_name":   idx.Field,
			"field_schema": idx.Type,
		}
		path := "/collections/" + c.cfg.Collection + "/index?wait=true"
		if err := c.put(ctx, path, body, nil); err != nil {
			return fmt.Errorf("create index %q: %w", idx.Field, err)
		}
	}
	return nil
}

// Upsert добавляет или обновляет batch точек (wait=true)
func (c *Client) Upsert(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	if len(points) > c.cfg.UpsertBatchSize {
		return fmt.Errorf("qdrant upsert: batch size %d exceeds limit %d",
			len(points), c.cfg.UpsertBatchSize)
	}

	items := make([]map[string]any, 0, len(points))
	for _, p := range points {
		items = append(items, map[string]any{
			"id":      p.ID,
			"vector":  p.Vector,
			"payload": p.Payload,
		})
	}
	body := map[string]any{"points": items}
	path := "/collections/" + c.cfg.Collection + "/points?wait=true"
	return c.put(ctx, path, body, nil)
}

// SearchHit одна точка-кандидат, возвращённая Qdrant.Search
type SearchHit struct {
	// ID идентификатор точки (dish_id)
	ID uint64
	// Score косинусное сходство (0..1, чем выше — тем лучше)
	Score float64
	// Payload payload точки
	Payload map[string]any
}

// Filter описывает payload-фильтр Qdrant; поля собираются как есть в JSON
type Filter struct {
	// Must все условия должны выполниться
	Must []FilterCondition `json:"must,omitempty"`
	// MustNot ни одно из условий не должно выполниться
	MustNot []FilterCondition `json:"must_not,omitempty"`
	// Should хотя бы одно из условий должно выполниться
	Should []FilterCondition `json:"should,omitempty"`
}

// FilterCondition условие на одно payload-поле
type FilterCondition struct {
	// Key имя payload-поля
	Key string `json:"key"`
	// Match условие на конкретное значение поля (eq)
	Match *FilterMatch `json:"match,omitempty"`
	// Range числовой range (gte/lte/...)
	Range *FilterRange `json:"range,omitempty"`
}

// FilterMatch условие на точное значение поля payload
type FilterMatch struct {
	// Value значение, с которым сравниваем (string | int | bool)
	Value any `json:"value"`
}

// FilterRange числовой range-фильтр
type FilterRange struct {
	// GT строго больше
	GT *float64 `json:"gt,omitempty"`
	// GTE больше или равно
	GTE *float64 `json:"gte,omitempty"`
	// LT строго меньше
	LT *float64 `json:"lt,omitempty"`
	// LTE меньше или равно
	LTE *float64 `json:"lte,omitempty"`
}

// Search ищет topK ближайших точек к vector с учётом фильтра
func (c *Client) Search(
	ctx context.Context,
	vector []float32,
	filter *Filter,
	topK int,
	withPayload bool,
) ([]SearchHit, error) {
	log := logger.ForCtx(ctx).With(
		"service", "qdrant",
		"op", "points/search",
		"collection", c.cfg.Collection,
		"top_k", topK,
		"filter_summary", filterSummary(filter),
	)
	body := map[string]any{
		"vector":       vector,
		"limit":        topK,
		"with_payload": withPayload,
	}
	if filter != nil {
		body["filter"] = filter
	}
	var resp struct {
		Result []struct {
			ID      uint64         `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	path := "/collections/" + c.cfg.Collection + "/points/search"

	start := time.Now()
	if err := c.post(ctx, path, body, &resp); err != nil {
		log.Error("qdrant search failed",
			"err", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, err
	}
	hits := make([]SearchHit, 0, len(resp.Result))
	for _, r := range resp.Result {
		hits = append(hits, SearchHit{ID: r.ID, Score: r.Score, Payload: r.Payload})
	}
	top := 0.0
	if len(hits) > 0 {
		top = hits[0].Score
	}
	log.Info("qdrant search ok",
		"hits", len(hits),
		"top_score", top,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return hits, nil
}

// filterSummary сворачивает фильтр в короткую строку для лога.
// Пример: must=is_available=true,category_id=5; must_not=allergens=dairy
func filterSummary(f *Filter) string {
	if f == nil {
		return ""
	}
	var parts []string
	if len(f.Must) > 0 {
		parts = append(parts, "must="+joinConditions(f.Must))
	}
	if len(f.MustNot) > 0 {
		parts = append(parts, "must_not="+joinConditions(f.MustNot))
	}
	if len(f.Should) > 0 {
		parts = append(parts, "should="+joinConditions(f.Should))
	}
	return strings.Join(parts, "; ")
}

// joinConditions сворачивает [{key=foo match=bar}, ...] в "foo=bar,baz=qux"
func joinConditions(cs []FilterCondition) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		switch {
		case c.Match != nil:
			out = append(out, fmt.Sprintf("%s=%v", c.Key, c.Match.Value))
		case c.Range != nil:
			out = append(out, c.Key+"=range")
		default:
			out = append(out, c.Key)
		}
	}
	return strings.Join(out, ",")
}

// CountPoints возвращает количество точек в коллекции
func (c *Client) CountPoints(ctx context.Context) (int, error) {
	var resp struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	body := map[string]any{"exact": true}
	if err := c.post(ctx, "/collections/"+c.cfg.Collection+"/points/count", body, &resp); err != nil {
		return 0, err
	}
	return resp.Result.Count, nil
}

// collectionExists проверяет, существует ли коллекция
func (c *Client) collectionExists(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.URL+"/collections/"+c.cfg.Collection, nil)
	if err != nil {
		return false, err
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("get collection: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf("get collection: status %d: %s", resp.StatusCode, string(errBody))
	}
}

// put шлёт PUT-запрос; out — опциональная структура для парсинга ответа
func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

// post шлёт POST-запрос
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	log := logger.ForCtx(ctx).With(
		"service", "qdrant",
		"method", method,
		"path", path,
	)
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.URL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuth(req)

	start := time.Now()
	resp, err := c.http.Do(req)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		log.Error("qdrant http call failed", "err", err, "duration_ms", dur)
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Error("qdrant http non-2xx",
			"status", resp.StatusCode,
			"body", strings.TrimSpace(string(errBody)),
			"duration_ms", dur,
		)
		return fmt.Errorf("qdrant %s %s: status %d: %s",
			method, path, resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	log.Debug("qdrant http call ok", "status", resp.StatusCode, "duration_ms", dur)

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.cfg.APIKey != "" {
		req.Header.Set("api-key", c.cfg.APIKey)
	}
}
