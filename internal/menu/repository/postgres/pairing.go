package postgres

import (
	"context"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/jackc/pgx/v5"
)

const pairingTagColumns = `slug, axis, label, embed_phrase, sort_order, is_active, created_at, updated_at`

const (
	listPairingTagsQuery = `
		SELECT ` + pairingTagColumns + `
		FROM pairing_tags
		ORDER BY axis, sort_order, slug`

	listActivePairingTagsByAxisQuery = `
		SELECT ` + pairingTagColumns + `
		FROM pairing_tags
		WHERE is_active
		ORDER BY axis, sort_order, slug`

	findPairingTagsBySlugsQuery = `
		SELECT ` + pairingTagColumns + `
		FROM pairing_tags
		WHERE slug = ANY($1)`

	pairingTagsForDishQuery = `
		SELECT pt.slug, pt.axis, pt.label, pt.embed_phrase, pt.sort_order, pt.is_active, pt.created_at, pt.updated_at
		FROM dish_pairing_tags dpt
		JOIN pairing_tags pt ON pt.slug = dpt.tag_slug
		WHERE dpt.dish_id = $1
		ORDER BY pt.axis, pt.sort_order, pt.slug`

	pairingTagsForDishesQuery = `
		SELECT dpt.dish_id,
		       pt.slug, pt.axis, pt.label, pt.embed_phrase, pt.sort_order, pt.is_active, pt.created_at, pt.updated_at
		FROM dish_pairing_tags dpt
		JOIN pairing_tags pt ON pt.slug = dpt.tag_slug
		WHERE dpt.dish_id = ANY($1)
		ORDER BY dpt.dish_id, pt.axis, pt.sort_order, pt.slug`

	deleteDishPairingTagsQuery = `DELETE FROM dish_pairing_tags WHERE dish_id = $1`
	insertDishPairingTagQuery  = `INSERT INTO dish_pairing_tags (dish_id, tag_slug) VALUES ($1, $2)`

	dishIDsByPairingTagQuery = `
		SELECT dish_id FROM dish_pairing_tags WHERE tag_slug = $1`
)

// ListPairingTags возвращает все pairing-теги (включая is_active=false) для админ UI.
func (r *Repository) ListPairingTags(ctx context.Context) ([]repositorymodels.PairingTag, error) {
	rows, err := r.pool.Query(ctx, listPairingTagsQuery)
	if err != nil {
		return nil, fmt.Errorf("query pairing_tags: %w", err)
	}
	defer rows.Close()
	return scanPairingTags(rows)
}

// ListActivePairingTags возвращает только теги с is_active=true; используется в
// валидации входящих slug'ов для PATCH /admin/dishes и в форме редактирования.
func (r *Repository) ListActivePairingTags(ctx context.Context) ([]repositorymodels.PairingTag, error) {
	rows, err := r.pool.Query(ctx, listActivePairingTagsByAxisQuery)
	if err != nil {
		return nil, fmt.Errorf("query active pairing_tags: %w", err)
	}
	defer rows.Close()
	return scanPairingTags(rows)
}

// FindPairingTagsBySlugs возвращает теги, чьи slug'и есть в списке. Порядок
// результата не гарантируется (использовать для валидации set'а, не для отображения).
func (r *Repository) FindPairingTagsBySlugs(
	ctx context.Context,
	slugs []string,
) ([]repositorymodels.PairingTag, error) {
	if len(slugs) == 0 {
		return []repositorymodels.PairingTag{}, nil
	}
	rows, err := r.pool.Query(ctx, findPairingTagsBySlugsQuery, slugs)
	if err != nil {
		return nil, fmt.Errorf("query pairing_tags by slugs: %w", err)
	}
	defer rows.Close()
	return scanPairingTags(rows)
}

// PairingTagsForDish возвращает теги одного блюда (отсортированы по axis+sort_order).
func (r *Repository) PairingTagsForDish(
	ctx context.Context,
	dishID int,
) ([]repositorymodels.PairingTag, error) {
	rows, err := r.pool.Query(ctx, pairingTagsForDishQuery, dishID)
	if err != nil {
		return nil, fmt.Errorf("query pairing_tags for dish: %w", err)
	}
	defer rows.Close()
	return scanPairingTags(rows)
}

// SetDishPairingTags переписывает связи блюда с pairing-тегами:
// удаляет старые, вставляет новые. Если slug несуществующего тега —
// возвращает ErrPairingTagNotFound. Делает это в транзакции.
func (r *Repository) SetDishPairingTags(
	ctx context.Context,
	dishID int,
	slugs []string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, deleteDishPairingTagsQuery, dishID); err != nil {
		return fmt.Errorf("clear dish_pairing_tags: %w", err)
	}
	for _, slug := range slugs {
		if _, err := tx.Exec(ctx, insertDishPairingTagQuery, dishID, slug); err != nil {
			if isForeignKeyViolation(err) {
				return menu.ErrPairingTagNotFound
			}
			return fmt.Errorf("insert dish_pairing_tag: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DishIDsByPairingTag возвращает id блюд, к которым привязан конкретный тег.
// Используется для каскадного реиндекса при изменении тега
// (embed_phrase у тега меняется → все блюда с ним нужно переэмбедить).
func (r *Repository) DishIDsByPairingTag(ctx context.Context, slug string) ([]int, error) {
	rows, err := r.pool.Query(ctx, dishIDsByPairingTagQuery, slug)
	if err != nil {
		return nil, fmt.Errorf("query dish_ids by pairing tag: %w", err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan dish_id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows dish_ids: %w", err)
	}
	return out, nil
}

func scanPairingTags(rows pgx.Rows) ([]repositorymodels.PairingTag, error) {
	out := []repositorymodels.PairingTag{}
	for rows.Next() {
		t, err := scanPairingTagRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows pairing_tags: %w", err)
	}
	return out, nil
}

func scanPairingTagRow(row pgx.Row) (*repositorymodels.PairingTag, error) {
	var t repositorymodels.PairingTag
	if err := row.Scan(
		&t.Slug, &t.Axis, &t.Label, &t.EmbedPhrase,
		&t.SortOrder, &t.IsActive,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

// attachPairingTags заполняет PairingTags у переданного слайса блюд одним
// SQL-запросом по списку их id (mirror of attachTags). Передавать только тех
// блюд, чьи id есть в dishIDs.
func (r *Repository) attachPairingTags(
	ctx context.Context,
	dishes []repositorymodels.Dish,
	dishIDs []int,
) error {
	if len(dishIDs) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx, pairingTagsForDishesQuery, dishIDs)
	if err != nil {
		return fmt.Errorf("query pairing_tags for dishes: %w", err)
	}
	defer rows.Close()

	tagsByDish := map[int][]repositorymodels.PairingTag{}
	for rows.Next() {
		var dishID int
		var t repositorymodels.PairingTag
		if err := rows.Scan(
			&dishID,
			&t.Slug, &t.Axis, &t.Label, &t.EmbedPhrase,
			&t.SortOrder, &t.IsActive,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return fmt.Errorf("scan dish_pairing_tag: %w", err)
		}
		tagsByDish[dishID] = append(tagsByDish[dishID], t)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows dish_pairing_tags: %w", err)
	}
	for i := range dishes {
		dishes[i].PairingTags = tagsByDish[dishes[i].ID]
	}
	return nil
}
