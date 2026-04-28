package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const categoryColumns = `id, name, sort_order, is_available, created_at, updated_at`

const (
	listCategoriesQuery = `
		SELECT ` + categoryColumns + `
		FROM categories
		ORDER BY sort_order, name`

	findCategoryByIDQuery = `
		SELECT ` + categoryColumns + `
		FROM categories
		WHERE id = $1`

	insertCategoryQuery = `
		INSERT INTO categories (name, sort_order, is_available)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	updateCategoryQuery = `
		UPDATE categories
		SET name = $2,
		    sort_order = $3,
		    is_available = $4,
		    updated_at = now()
		WHERE id = $1`

	countDishesInCategoryQuery = `SELECT COUNT(*) FROM dishes WHERE category_id = $1`
	deleteCategoryQuery        = `DELETE FROM categories WHERE id = $1`
)

// ListCategories возвращает все категории
func (r *Repository) ListCategories(ctx context.Context) ([]repositorymodels.Category, error) {
	rows, err := r.pool.Query(ctx, listCategoriesQuery)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var out []repositorymodels.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows categories: %w", err)
	}
	return out, nil
}

// FindCategoryByID возвращает категорию
func (r *Repository) FindCategoryByID(ctx context.Context, id int) (*repositorymodels.Category, error) {
	row := r.pool.QueryRow(ctx, findCategoryByIDQuery, id)
	c, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, menu.ErrCategoryNotFound
	}
	return c, err
}

// CreateCategory вставляет категорию
func (r *Repository) CreateCategory(ctx context.Context, c *repositorymodels.Category) error {
	err := r.pool.QueryRow(ctx, insertCategoryQuery, c.Name, c.SortOrder, c.IsAvailable).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "categories_name_key") {
			return menu.ErrCategoryNameTaken
		}
		return fmt.Errorf("insert category: %w", err)
	}
	return nil
}

// UpdateCategory сохраняет изменения категории
func (r *Repository) UpdateCategory(ctx context.Context, c *repositorymodels.Category) error {
	cmd, err := r.pool.Exec(ctx, updateCategoryQuery, c.ID, c.Name, c.SortOrder, c.IsAvailable)
	if err != nil {
		if isUniqueViolation(err, "categories_name_key") {
			return menu.ErrCategoryNameTaken
		}
		return fmt.Errorf("update category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrCategoryNotFound
	}
	return nil
}

// DeleteCategory удаляет категорию, если в ней нет блюд
func (r *Repository) DeleteCategory(ctx context.Context, id int) error {
	var n int
	if err := r.pool.QueryRow(ctx, countDishesInCategoryQuery, id).Scan(&n); err != nil {
		return fmt.Errorf("count dishes: %w", err)
	}
	if n > 0 {
		return menu.ErrCategoryHasDishes
	}
	cmd, err := r.pool.Exec(ctx, deleteCategoryQuery, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrCategoryNotFound
	}
	return nil
}

func scanCategory(row pgx.Row) (*repositorymodels.Category, error) {
	var c repositorymodels.Category
	if err := row.Scan(&c.ID, &c.Name, &c.SortOrder, &c.IsAvailable, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != pgerrcode.UniqueViolation {
		return false
	}
	if constraint == "" {
		return true
	}
	return pgErr.ConstraintName == constraint
}
