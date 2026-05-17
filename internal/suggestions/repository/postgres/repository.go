// Package postgres реализует suggestions.Repository поверх PostgreSQL (pgx/v5).
package postgres

import (
	"context"
	"errors"
	"fmt"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/suggestions"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository PostgreSQL-репозиторий подсказок чата.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const suggestionColumns = `id, text, sort_order, is_active, clicks_count, created_at, updated_at`

const (
	listActiveQuery = `
		SELECT ` + suggestionColumns + `
		FROM chat_suggestions
		WHERE is_active = true
		ORDER BY sort_order, id`

	listAllQuery = `
		SELECT ` + suggestionColumns + `
		FROM chat_suggestions
		ORDER BY sort_order, id`

	findByIDQuery = `
		SELECT ` + suggestionColumns + `
		FROM chat_suggestions
		WHERE id = $1`

	insertQuery = `
		INSERT INTO chat_suggestions (text, sort_order, is_active)
		VALUES ($1, $2, $3)
		RETURNING id, clicks_count, created_at, updated_at`

	updateQuery = `
		UPDATE chat_suggestions
		SET text = $2, sort_order = $3, is_active = $4, updated_at = now()
		WHERE id = $1`

	deleteQuery = `DELETE FROM chat_suggestions WHERE id = $1`

	incClicksQuery = `UPDATE chat_suggestions SET clicks_count = clicks_count + 1 WHERE id = $1`
)

// ListActive возвращает активные подсказки.
func (r *Repository) ListActive(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error) {
	return r.query(ctx, listActiveQuery)
}

// ListAll возвращает все подсказки.
func (r *Repository) ListAll(ctx context.Context) ([]usecasemodels.AdminChatSuggestion, error) {
	return r.query(ctx, listAllQuery)
}

// FindByID находит подсказку.
func (r *Repository) FindByID(ctx context.Context, id int64) (*usecasemodels.AdminChatSuggestion, error) {
	row := r.pool.QueryRow(ctx, findByIDQuery, id)
	s, err := scanSuggestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, suggestions.ErrNotFound
	}
	return s, err
}

// Create вставляет подсказку.
func (r *Repository) Create(ctx context.Context, c *usecasemodels.AdminChatSuggestion) error {
	err := r.pool.QueryRow(ctx, insertQuery, c.Text, c.SortOrder, c.IsActive).
		Scan(&c.ID, &c.ClicksCount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert chat_suggestion: %w", err)
	}
	return nil
}

// Update обновляет подсказку. Поле clicks_count руками не трогаем — только через IncrementClicks.
func (r *Repository) Update(ctx context.Context, c *usecasemodels.AdminChatSuggestion) error {
	cmd, err := r.pool.Exec(ctx, updateQuery, c.ID, c.Text, c.SortOrder, c.IsActive)
	if err != nil {
		return fmt.Errorf("update chat_suggestion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return suggestions.ErrNotFound
	}
	return nil
}

// Delete удаляет подсказку.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	cmd, err := r.pool.Exec(ctx, deleteQuery, id)
	if err != nil {
		return fmt.Errorf("delete chat_suggestion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return suggestions.ErrNotFound
	}
	return nil
}

// IncrementClicks увеличивает счётчик кликов на 1.
func (r *Repository) IncrementClicks(ctx context.Context, id int64) error {
	cmd, err := r.pool.Exec(ctx, incClicksQuery, id)
	if err != nil {
		return fmt.Errorf("inc clicks: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return suggestions.ErrNotFound
	}
	return nil
}

func (r *Repository) query(ctx context.Context, sql string) ([]usecasemodels.AdminChatSuggestion, error) {
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query chat_suggestions: %w", err)
	}
	defer rows.Close()
	var out []usecasemodels.AdminChatSuggestion
	for rows.Next() {
		s, scanErr := scanSuggestion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows chat_suggestions: %w", err)
	}
	return out, nil
}

func scanSuggestion(row pgx.Row) (*usecasemodels.AdminChatSuggestion, error) {
	var s usecasemodels.AdminChatSuggestion
	if err := row.Scan(&s.ID, &s.Text, &s.SortOrder, &s.IsActive, &s.ClicksCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}
