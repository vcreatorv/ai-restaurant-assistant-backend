package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/jackc/pgx/v5"
)

const tagColumns = `id, name, slug, color, created_at, updated_at`

const (
	listTagsQuery = `
		SELECT ` + tagColumns + `
		FROM tags
		ORDER BY name`

	findTagByIDQuery = `
		SELECT ` + tagColumns + `
		FROM tags
		WHERE id = $1`

	findTagsByIDsQuery = `
		SELECT ` + tagColumns + `
		FROM tags
		WHERE id = ANY($1)`

	insertTagQuery = `
		INSERT INTO tags (name, slug, color)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	updateTagQuery = `
		UPDATE tags
		SET name = $2,
		    slug = $3,
		    color = $4,
		    updated_at = now()
		WHERE id = $1`

	deleteTagQuery = `DELETE FROM tags WHERE id = $1`
)

// ListTags возвращает все теги
func (r *Repository) ListTags(ctx context.Context) ([]repositorymodels.Tag, error) {
	rows, err := r.pool.Query(ctx, listTagsQuery)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

// FindTagsByIDs возвращает теги по списку идентификаторов
func (r *Repository) FindTagsByIDs(ctx context.Context, ids []int) ([]repositorymodels.Tag, error) {
	if len(ids) == 0 {
		return []repositorymodels.Tag{}, nil
	}
	rows, err := r.pool.Query(ctx, findTagsByIDsQuery, ids)
	if err != nil {
		return nil, fmt.Errorf("query tags by ids: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

// FindTagByID возвращает тег
func (r *Repository) FindTagByID(ctx context.Context, id int) (*repositorymodels.Tag, error) {
	row := r.pool.QueryRow(ctx, findTagByIDQuery, id)
	t, err := scanTagRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, menu.ErrTagNotFound
	}
	return t, err
}

// CreateTag вставляет тег
func (r *Repository) CreateTag(ctx context.Context, t *repositorymodels.Tag) error {
	err := r.pool.QueryRow(ctx, insertTagQuery, t.Name, t.Slug, t.Color).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "") {
			return menu.ErrTagNameTaken
		}
		return fmt.Errorf("insert tag: %w", err)
	}
	return nil
}

// UpdateTag сохраняет изменения тега
func (r *Repository) UpdateTag(ctx context.Context, t *repositorymodels.Tag) error {
	cmd, err := r.pool.Exec(ctx, updateTagQuery, t.ID, t.Name, t.Slug, t.Color)
	if err != nil {
		if isUniqueViolation(err, "") {
			return menu.ErrTagNameTaken
		}
		return fmt.Errorf("update tag: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrTagNotFound
	}
	return nil
}

// DeleteTag удаляет тег
func (r *Repository) DeleteTag(ctx context.Context, id int) error {
	cmd, err := r.pool.Exec(ctx, deleteTagQuery, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrTagNotFound
	}
	return nil
}

func scanTags(rows pgx.Rows) ([]repositorymodels.Tag, error) {
	var out []repositorymodels.Tag
	for rows.Next() {
		t, err := scanTagRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows tags: %w", err)
	}
	return out, nil
}

func scanTagRow(row pgx.Row) (*repositorymodels.Tag, error) {
	var t repositorymodels.Tag
	if err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}
