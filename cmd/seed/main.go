// Package main — одноразовый идемпотентный сидер меню из seed/menu.json.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/cmd/app/app"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Seed корневая структура seed/menu.json
type Seed struct {
	Categories []SeedCategory `json:"categories"`
	Tags       []SeedTag      `json:"tags"`
	Dishes     []SeedDish     `json:"dishes"`
}

// SeedCategory категория из seed
type SeedCategory struct {
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	IsAvailable bool   `json:"is_available"`
}

// SeedTag тег из seed
type SeedTag struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
}

// SeedDish блюдо из seed
type SeedDish struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Composition      string   `json:"composition"`
	ImageURLExternal string   `json:"image_url_external"`
	ImageKey         string   `json:"image_key"`
	PriceMinor       int      `json:"price_minor"`
	Currency         string   `json:"currency"`
	CaloriesKcal     *int     `json:"calories_kcal"`
	ProteinG         *float64 `json:"protein_g"`
	FatG             *float64 `json:"fat_g"`
	CarbsG           *float64 `json:"carbs_g"`
	PortionWeightG   *int     `json:"portion_weight_g"`
	Cuisine          string   `json:"cuisine"`
	Category         string   `json:"category"`
	Allergens        []string `json:"allergens"`
	Dietary          []string `json:"dietary"`
	TagSlugs         []string `json:"tag_slugs"`
	IsAvailable      bool     `json:"is_available"`
	Synthetic        bool     `json:"synthetic"`
}

// drinkCategories категории, для которых при отсутствии своей картинки берём drink-плейсхолдер
var drinkCategories = map[string]bool{
	"Напитки безалкогольные": true,
	"Напитки алкогольные":    true,
}

func main() {
	os.Exit(run())
}

func run() int {
	cfgPath := flag.String("config", app.DefaultConfigPath, "path to yaml config")
	seedPath := flag.String("seed", "seed/menu.json", "path to seed/menu.json")
	assetsDir := flag.String("assets", "seed/assets", "path to assets dir (default images)")
	skipImages := flag.Bool("skip-images", false, "skip downloading external images (use defaults / empty)")
	flag.Parse()

	cfg, err := app.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("config", "err", err)
		return 1
	}

	ctx := context.Background()

	pool, err := datasources.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		slog.Error("postgres", "err", err)
		return 1
	}
	defer pool.Close()

	storage, err := s3.New(cfg.S3)
	if err != nil {
		slog.Error("s3", "err", err)
		return 1
	}

	data, err := loadSeed(*seedPath)
	if err != nil {
		slog.Error("load seed", "err", err)
		return 1
	}

	r := &runner{
		pool:    pool,
		storage: storage,
		http:    &http.Client{Timeout: 30 * time.Second},
		skipImg: *skipImages,
	}

	r.uploadDefaults(ctx, *assetsDir)

	catIDs, err := r.upsertCategories(ctx, data.Categories)
	if err != nil {
		slog.Error("categories", "err", err)
		return 1
	}
	tagIDs, err := r.upsertTags(ctx, data.Tags)
	if err != nil {
		slog.Error("tags", "err", err)
		return 1
	}
	if err := r.upsertDishes(ctx, data.Dishes, catIDs, tagIDs); err != nil {
		slog.Error("dishes", "err", err)
		return 1
	}
	slog.Info("seed done",
		"categories", len(catIDs),
		"tags", len(tagIDs),
		"dishes_inserted", r.dishesInserted,
		"dishes_skipped", r.dishesSkipped,
		"images_uploaded", r.imagesUploaded,
		"images_reused", r.imagesReused,
		"images_failed", r.imagesFailed,
	)
	return 0
}

func loadSeed(path string) (*Seed, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path задаётся оператором через флаг
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Seed
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

type runner struct {
	pool    *pgxpool.Pool
	storage s3.Storage
	http    *http.Client
	skipImg bool

	defaultDishURL  string
	defaultDrinkURL string

	dishesInserted int
	dishesSkipped  int
	imagesUploaded int
	imagesReused   int
	imagesFailed   int
}

// uploadDefaults заливает дефолтные картинки из assets-директории, если они там есть
func (r *runner) uploadDefaults(ctx context.Context, assetsDir string) {
	dish := filepath.Join(assetsDir, "default_dish.png")
	drink := filepath.Join(assetsDir, "default_drink.png")
	r.defaultDishURL = r.uploadAsset(ctx, dish, "defaults/dish.png", "image/png")
	r.defaultDrinkURL = r.uploadAsset(ctx, drink, "defaults/drink.png", "image/png")
}

func (r *runner) uploadAsset(ctx context.Context, path, key, contentType string) string {
	f, err := os.Open(path) //nolint:gosec // путь — наш asset
	if err != nil {
		slog.Warn("default asset missing", "path", path)
		return ""
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		slog.Warn("default asset stat", "path", path, "err", err)
		return ""
	}
	url, err := r.storage.Upload(ctx, key, contentType, f, st.Size())
	if err != nil {
		slog.Warn("default asset upload", "path", path, "err", err)
		return ""
	}
	slog.Info("default asset uploaded", "key", key, "url", url)
	return url
}

func (r *runner) upsertCategories(ctx context.Context, items []SeedCategory) (map[string]int, error) {
	out := make(map[string]int, len(items))
	for _, c := range items {
		var id int
		err := r.pool.QueryRow(ctx, `
			INSERT INTO categories (name, sort_order, is_available)
			VALUES ($1, $2, $3)
			ON CONFLICT (name) DO UPDATE
			  SET sort_order = EXCLUDED.sort_order,
			      is_available = EXCLUDED.is_available,
			      updated_at = now()
			RETURNING id`, c.Name, c.SortOrder, c.IsAvailable).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("upsert category %q: %w", c.Name, err)
		}
		out[c.Name] = id
	}
	return out, nil
}

func (r *runner) upsertTags(ctx context.Context, items []SeedTag) (map[string]int, error) {
	out := make(map[string]int, len(items))
	for _, t := range items {
		var id int
		err := r.pool.QueryRow(ctx, `
			INSERT INTO tags (name, slug, color)
			VALUES ($1, $2, $3)
			ON CONFLICT (slug) DO UPDATE
			  SET name = EXCLUDED.name,
			      color = EXCLUDED.color,
			      updated_at = now()
			RETURNING id`, t.Name, t.Slug, t.Color).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("upsert tag %q: %w", t.Slug, err)
		}
		out[t.Slug] = id
	}
	return out, nil
}

func (r *runner) upsertDishes(ctx context.Context, items []SeedDish, catIDs, tagIDs map[string]int) error {
	for i := range items {
		d := &items[i]
		catID, ok := catIDs[d.Category]
		if !ok {
			return fmt.Errorf("dish %q: unknown category %q", d.Name, d.Category)
		}

		var existingID int
		err := r.pool.QueryRow(ctx, `SELECT id FROM dishes WHERE name = $1`, d.Name).Scan(&existingID)
		if err == nil {
			r.dishesSkipped++
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check dish %q: %w", d.Name, err)
		}

		imageURL := r.resolveImage(ctx, d)

		var id int
		err = r.pool.QueryRow(ctx, `
			INSERT INTO dishes (
				name, description, composition, image_url,
				price_minor, currency,
				calories_kcal, protein_g, fat_g, carbs_g, portion_weight_g,
				cuisine, category_id,
				allergens, dietary,
				is_available
			) VALUES (
				$1, $2, $3, $4,
				$5, $6,
				$7, $8, $9, $10, $11,
				$12, $13,
				$14, $15,
				$16
			)
			RETURNING id`,
			d.Name, d.Description, d.Composition, imageURL,
			d.PriceMinor, defaultStr(d.Currency, "RUB"),
			d.CaloriesKcal, d.ProteinG, d.FatG, d.CarbsG, d.PortionWeightG,
			d.Cuisine, catID,
			d.Allergens, d.Dietary,
			d.IsAvailable,
		).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert dish %q: %w", d.Name, err)
		}

		for _, slug := range d.TagSlugs {
			tagID, ok := tagIDs[slug]
			if !ok {
				slog.Warn("unknown tag, skipped", "dish", d.Name, "tag", slug)
				continue
			}
			if _, err := r.pool.Exec(ctx,
				`INSERT INTO dish_tags (dish_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, tagID,
			); err != nil {
				return fmt.Errorf("link dish %q tag %q: %w", d.Name, slug, err)
			}
		}

		r.dishesInserted++
	}
	return nil
}

// resolveImage возвращает финальный URL картинки: либо из MinIO (своя или ранее залитая),
// либо дефолт по типу категории, либо пустая строка
func (r *runner) resolveImage(ctx context.Context, d *SeedDish) string {
	if !r.skipImg && d.ImageKey != "" && d.ImageURLExternal != "" {
		exists, _ := r.storage.Exists(ctx, d.ImageKey)
		if exists {
			r.imagesReused++
			return r.storage.URL(d.ImageKey)
		}
		url, err := r.downloadAndUpload(ctx, d.ImageURLExternal, d.ImageKey)
		if err == nil {
			r.imagesUploaded++
			return url
		}
		r.imagesFailed++
		slog.Warn("image download failed, using default", "dish", d.Name, "err", err)
	}
	if drinkCategories[d.Category] {
		return r.defaultDrinkURL
	}
	return r.defaultDishURL
}

func (r *runner) downloadAndUpload(ctx context.Context, src, key string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	const maxBytes = 10 * 1024 * 1024
	buf := &bytes.Buffer{}
	n, err := io.Copy(buf, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if n > maxBytes {
		return "", fmt.Errorf("image too large: >%d", maxBytes)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return r.storage.Upload(ctx, key, contentType, buf, n)
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
