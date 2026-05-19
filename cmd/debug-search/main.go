// Package main — CLI-инструмент для отладки vector-search'а блюд в Qdrant.
//
// Использование:
//
//	go run ./cmd/debug-search -q "подойдёт под белое вино" -n 15
//
// Что делает: эмбеддит запрос через Cohere (тот же клиент и модель, что и runtime),
// ищет top-N в Qdrant без всяких фильтров и рерэнка, дочитывает имена из PG.
// Это «как retrieval видит запрос» — без classifier, без rerank, без diversify,
// без companion-логики. Помогает понять, на каком этапе теряется качество.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/cmd/app/app"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/cohere"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfgPath := flag.String("config", app.DefaultConfigPath, "path to yaml config")
	query := flag.String("q", "", "query text")
	limit := flag.Int("n", 10, "top-N")
	tag := flag.String("tag", "", "pairing tag slug for stream B filter (e.g. pair_white_wine)")
	rrf := flag.Bool("rrf", false, "если указан -tag, делать hybrid retrieval с RRF (A + B + merge)")
	cat := flag.Int("cat", 0, "category_id для hard-фильтра (cross-sell: «гарнир к стейку» → 10)")
	price := flag.String("price", "", "ценовой intent: cheap|premium (фильтр по price_minor)")
	flag.Parse()

	if *query == "" {
		slog.Error("query is required (-q)")
		return 2
	}

	cfg, err := app.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("config", "err", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := datasources.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		slog.Error("postgres", "err", err)
		return 1
	}
	defer pool.Close()

	cohereClient, err := cohere.New(cfg.RAG.Cohere)
	if err != nil {
		slog.Error("cohere", "err", err)
		return 1
	}
	qdrantClient := qdrant.New(cfg.RAG.Qdrant)

	vectors, err := cohereClient.Embed(ctx, []string{*query}, rag.CohereInputQuery)
	if err != nil {
		slog.Error("embed query", "err", err)
		return 1
	}
	if len(vectors) == 0 {
		slog.Error("empty embed result")
		return 1
	}
	vec := vectors[0]

	// Базовый фильтр Stream A: только is_available + опц. category_id + опц. price range.
	// Симулирует то, что делает chat.runRAG c retrievalIntent (без user-restrictions —
	// debug-инструмент идёт без профиля).
	baseFilter := &qdrant.Filter{
		Must: []qdrant.FilterCondition{
			{Key: "is_available", Match: &qdrant.FilterMatch{Value: true}},
		},
	}
	if *cat > 0 {
		baseFilter.Must = append(baseFilter.Must, qdrant.FilterCondition{
			Key: "category_id", Match: &qdrant.FilterMatch{Value: *cat},
		})
	}
	switch *price {
	case "cheap":
		v := 150000.0
		baseFilter.Must = append(baseFilter.Must, qdrant.FilterCondition{
			Key: "price_minor", Range: &qdrant.FilterRange{LTE: &v},
		})
	case "premium":
		v := 250000.0
		baseFilter.Must = append(baseFilter.Must, qdrant.FilterCondition{
			Key: "price_minor", Range: &qdrant.FilterRange{GTE: &v},
		})
	}

	// Stream A: vector-search с базовым фильтром.
	hitsA, err := qdrantClient.Search(ctx, vec, baseFilter, *limit, false)
	if err != nil {
		slog.Error("qdrant search A", "err", err)
		return 1
	}

	// Stream B (опционально): то же что A + pairing-tag must.
	var hitsB []qdrant.SearchHit
	if *tag != "" {
		filterB := &qdrant.Filter{
			Must: append([]qdrant.FilterCondition(nil), baseFilter.Must...),
		}
		filterB.Must = append(filterB.Must, qdrant.FilterCondition{
			Key: "pairing_tags", Match: &qdrant.FilterMatch{Value: *tag},
		})
		hitsB, err = qdrantClient.Search(ctx, vec, filterB, *limit, false)
		if err != nil {
			slog.Error("qdrant search B", "err", err)
			return 1
		}
	}

	// Решаем, что показывать.
	if *tag == "" {
		printStream(ctx, pool, *query, "Stream A: pure vector search (no filter)", hitsA, *limit)
		return 0
	}
	printStream(ctx, pool, *query, fmt.Sprintf("Stream A: pure vector search (no filter, %d hits)", len(hitsA)), hitsA, *limit)
	printStream(ctx, pool, *query, fmt.Sprintf("Stream B: vector search WHERE pairing_tags=%s (%d hits)", *tag, len(hitsB)), hitsB, *limit)

	if *rrf {
		merged := mergeRRF(hitsA, hitsB, *limit)
		printStream(ctx, pool, *query, fmt.Sprintf("MERGED via RRF (k=%d)", rrfK), merged, *limit)
	}
	return 0
}

// rrfK — константа сглаживания, такая же как в chat usecase (Cormack et al. 2009).
const rrfK = 60

// mergeRRF — копия логики из chat/usecase/chat.go: для каждого hit
// score = Σ 1/(k + rank + 1) по обоим потокам.
func mergeRRF(a, b []qdrant.SearchHit, topN int) []qdrant.SearchHit {
	type acc struct {
		score float64
		base  qdrant.SearchHit
	}
	bucket := make(map[uint64]*acc, len(a)+len(b))
	add := func(stream []qdrant.SearchHit) {
		for rank, h := range stream {
			e, ok := bucket[h.ID]
			if !ok {
				e = &acc{base: h}
				bucket[h.ID] = e
			}
			e.score += 1.0 / float64(rrfK+rank+1)
		}
	}
	add(a)
	add(b)
	out := make([]qdrant.SearchHit, 0, len(bucket))
	for _, e := range bucket {
		out = append(out, qdrant.SearchHit{ID: e.base.ID, Score: e.score})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out
}

// printStream печатает таблицу с топом блюд (дочитывая имена из PG).
func printStream(
	ctx context.Context,
	pool *pgxpool.Pool,
	query, header string,
	hits []qdrant.SearchHit,
	limit int,
) {
	if len(hits) > limit {
		hits = hits[:limit]
	}
	rows, err := loadNames(ctx, pool, hits)
	if err != nil {
		slog.Error("load names", "err", err)
		return
	}
	fmt.Printf("\nQuery: %q\n", query)
	fmt.Printf("%s\n", header)
	fmt.Printf("%-4s %-8s %-34s %-18s %s\n", "#", "score", "name", "category", "id")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────────")
	for i, h := range hits {
		id := int(h.ID) //nolint:gosec
		r := rows[id]
		fmt.Printf("%-4d %.4f   %-34s %-18s %d\n", i+1, h.Score, truncate(r.name, 34), truncate(r.cat, 18), id)
	}
}

type dishRow struct {
	name string
	cat  string
}

func loadNames(ctx context.Context, pool *pgxpool.Pool, hits []qdrant.SearchHit) (map[int]dishRow, error) {
	if len(hits) == 0 {
		return map[int]dishRow{}, nil
	}
	ids := make([]int, len(hits))
	for i, h := range hits {
		ids[i] = int(h.ID) //nolint:gosec
	}
	const q = `SELECT d.id, d.name, c.name FROM dishes d JOIN categories c ON c.id = d.category_id WHERE d.id = ANY($1)`
	rows, err := pool.Query(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]dishRow{}
	for rows.Next() {
		var id int
		var r dishRow
		if err := rows.Scan(&id, &r.name, &r.cat); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
