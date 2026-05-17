// Package postgres реализует prompts.Repository поверх pgx.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository PostgreSQL-репозиторий промптов.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const (
	versionColumns = `
		p.version, p.content, p.published_at,
		u.id AS author_id,
		COALESCE(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.email) AS author_name,
		u.email AS author_email`

	findActiveVersionQuery = `
		SELECT ` + versionColumns + `
		FROM prompts p
		JOIN users u ON u.id = p.published_by
		WHERE p.name = $1
		ORDER BY p.version DESC
		LIMIT 1`

	findActiveContentQuery = `
		SELECT content FROM prompts WHERE name = $1 ORDER BY version DESC LIMIT 1`

	listVersionsQuery = `
		SELECT ` + versionColumns + `
		FROM prompts p
		JOIN users u ON u.id = p.published_by
		WHERE p.name = $1
		ORDER BY p.version DESC`

	findVersionQuery = `
		SELECT ` + versionColumns + `
		FROM prompts p
		JOIN users u ON u.id = p.published_by
		WHERE p.name = $1 AND p.version = $2`

	insertVersionQuery = `
		INSERT INTO prompts (name, version, content, published_by)
		VALUES (
		    $1,
		    COALESCE((SELECT MAX(version) FROM prompts WHERE name = $1), 0) + 1,
		    $2,
		    $3
		)
		RETURNING version, published_at`

	deleteDraftQuery = `DELETE FROM prompt_drafts WHERE admin_id = $1 AND prompt_name = $2`

	findAuthorQuery = `
		SELECT id,
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', first_name, last_name)), ''), email) AS display_name,
		       email
		FROM users WHERE id = $1`

	findDraftQuery = `
		SELECT prompt_name, content, updated_at
		FROM prompt_drafts
		WHERE admin_id = $1 AND prompt_name = $2`

	findDraftContentQuery = `
		SELECT content FROM prompt_drafts WHERE admin_id = $1 AND prompt_name = $2`

	upsertDraftQuery = `
		INSERT INTO prompt_drafts (admin_id, prompt_name, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (admin_id, prompt_name)
		DO UPDATE SET content = EXCLUDED.content, updated_at = now()
		RETURNING prompt_name, content, updated_at`
)

// FindActiveVersion возвращает запись с максимальным version по name.
func (r *Repository) FindActiveVersion(ctx context.Context, name prompts.Name) (*prompts.Version, error) {
	row := r.pool.QueryRow(ctx, findActiveVersionQuery, string(name))
	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, prompts.ErrNotFound
	}
	return v, err
}

// FindActiveContent возвращает только content активной версии — для горячего пути чата.
func (r *Repository) FindActiveContent(ctx context.Context, name prompts.Name) (string, error) {
	var content string
	err := r.pool.QueryRow(ctx, findActiveContentQuery, string(name)).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", prompts.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find active content: %w", err)
	}
	return content, nil
}

// ListVersions возвращает все версии по name по убыванию version.
func (r *Repository) ListVersions(ctx context.Context, name prompts.Name) ([]prompts.Version, error) {
	rows, err := r.pool.Query(ctx, listVersionsQuery, string(name))
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	out := []prompts.Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows versions: %w", err)
	}
	return out, nil
}

// FindVersion возвращает конкретную версию.
func (r *Repository) FindVersion(ctx context.Context, name prompts.Name, version int) (*prompts.Version, error) {
	row := r.pool.QueryRow(ctx, findVersionQuery, string(name), version)
	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, prompts.ErrVersionNotFound
	}
	return v, err
}

// InsertVersion создаёт новую запись с version = max+1 атомарно.
// Если deleteDraft == true, в той же транзакции удаляет черновик автора.
func (r *Repository) InsertVersion(
	ctx context.Context,
	name prompts.Name,
	content string,
	authorID uuid.UUID,
	deleteDraft bool,
) (*prompts.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	v := &prompts.Version{Content: content}
	if err := tx.QueryRow(ctx, insertVersionQuery, string(name), content, authorID).
		Scan(&v.Version, &v.PublishedAt); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if deleteDraft {
		if _, err := tx.Exec(ctx, deleteDraftQuery, authorID, string(name)); err != nil {
			return nil, fmt.Errorf("delete draft on publish: %w", err)
		}
	}

	if err := tx.QueryRow(ctx, findAuthorQuery, authorID).
		Scan(&v.PublishedBy.ID, &v.PublishedBy.DisplayName, &v.PublishedBy.Email); err != nil {
		return nil, fmt.Errorf("load author: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return v, nil
}

// FindDraft возвращает черновик админа.
func (r *Repository) FindDraft(ctx context.Context, adminID uuid.UUID, name prompts.Name) (*prompts.Draft, error) {
	row := r.pool.QueryRow(ctx, findDraftQuery, adminID, string(name))
	var d prompts.Draft
	var n string
	if err := row.Scan(&n, &d.Content, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find draft: %w", err)
	}
	d.Name = prompts.Name(n)
	return &d, nil
}

// FindDraftContent возвращает только content черновика. Второе значение — был ли найден.
func (r *Repository) FindDraftContent(ctx context.Context, adminID uuid.UUID, name prompts.Name) (string, bool, error) {
	var content string
	err := r.pool.QueryRow(ctx, findDraftContentQuery, adminID, string(name)).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find draft content: %w", err)
	}
	return content, true, nil
}

// UpsertDraft создаёт или обновляет черновик.
func (r *Repository) UpsertDraft(
	ctx context.Context,
	adminID uuid.UUID,
	name prompts.Name,
	content string,
) (*prompts.Draft, error) {
	row := r.pool.QueryRow(ctx, upsertDraftQuery, adminID, string(name), content)
	var d prompts.Draft
	var n string
	if err := row.Scan(&n, &d.Content, &d.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert draft: %w", err)
	}
	d.Name = prompts.Name(n)
	return &d, nil
}

// DeleteDraft удаляет черновик (idempotent).
func (r *Repository) DeleteDraft(ctx context.Context, adminID uuid.UUID, name prompts.Name) error {
	if _, err := r.pool.Exec(ctx, deleteDraftQuery, adminID, string(name)); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}

// scanRow абстракция над pgx.Row / pgx.Rows для общего сканера.
type scanRow interface {
	Scan(dest ...any) error
}

// scanVersion разбирает строку версии (см. versionColumns).
func scanVersion(row scanRow) (*prompts.Version, error) {
	var v prompts.Version
	if err := row.Scan(
		&v.Version, &v.Content, &v.PublishedAt,
		&v.PublishedBy.ID, &v.PublishedBy.DisplayName, &v.PublishedBy.Email,
	); err != nil {
		return nil, err
	}
	return &v, nil
}
