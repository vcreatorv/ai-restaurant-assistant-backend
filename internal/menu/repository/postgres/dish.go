package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/jackc/pgx/v5"
)

const dishColumns = `
	id, name, description, composition, image_url,
	price_minor, currency,
	calories_kcal, protein_g, fat_g, carbs_g, portion_weight_g,
	cuisine, category_id,
	allergens, dietary,
	is_available,
	created_at, updated_at`

const (
	findDishByIDQuery = `
		SELECT ` + dishColumns + `
		FROM dishes
		WHERE id = $1`

	insertDishQuery = `
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
		RETURNING id, created_at, updated_at`

	updateDishQuery = `
		UPDATE dishes SET
			name = $2,
			description = $3,
			composition = $4,
			image_url = $5,
			price_minor = $6,
			currency = $7,
			calories_kcal = $8,
			protein_g = $9,
			fat_g = $10,
			carbs_g = $11,
			portion_weight_g = $12,
			cuisine = $13,
			category_id = $14,
			allergens = $15,
			dietary = $16,
			is_available = $17,
			updated_at = now()
		WHERE id = $1`

	setDishAvailabilityQuery = `
		UPDATE dishes SET is_available = $2, updated_at = now()
		WHERE id = $1`

	dishTagsForDishesQuery = `
		SELECT dish_id, ` + tagColumnsPrefixed + `
		FROM dish_tags
		JOIN tags ON tags.id = dish_tags.tag_id
		WHERE dish_id = ANY($1)
		ORDER BY tags.name`

	deleteDishTagsQuery = `DELETE FROM dish_tags WHERE dish_id = $1`
	insertDishTagQuery  = `INSERT INTO dish_tags (dish_id, tag_id) VALUES ($1, $2)`
)

const tagColumnsPrefixed = `
	tags.id, tags.name, tags.slug, tags.color, tags.created_at, tags.updated_at`

// ListDishes возвращает блюда по фильтру с пагинацией и общим count
func (r *Repository) ListDishes(
	ctx context.Context,
	f repositorymodels.DishFilter,
) ([]repositorymodels.Dish, int, error) {
	where, args := buildDishWhere(f)

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	listQuery := `SELECT ` + dishColumns + ` FROM dishes ` + where +
		` ORDER BY id ` +
		` LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	listArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query dishes: %w", err)
	}
	defer rows.Close()

	var dishes []repositorymodels.Dish
	dishIDs := make([]int, 0)
	for rows.Next() {
		d, err := scanDish(rows)
		if err != nil {
			return nil, 0, err
		}
		dishes = append(dishes, *d)
		dishIDs = append(dishIDs, d.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows dishes: %w", err)
	}

	if err := r.attachTags(ctx, dishes, dishIDs); err != nil {
		return nil, 0, err
	}

	countQuery := `SELECT COUNT(*) FROM dishes ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count dishes: %w", err)
	}

	return dishes, total, nil
}

// FindDishByID возвращает блюдо с тегами
func (r *Repository) FindDishByID(ctx context.Context, id int) (*repositorymodels.Dish, error) {
	row := r.pool.QueryRow(ctx, findDishByIDQuery, id)
	d, err := scanDish(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, menu.ErrDishNotFound
	}
	if err != nil {
		return nil, err
	}
	tags, err := r.tagsForDish(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Tags = tags
	return d, nil
}

// CreateDish вставляет блюдо и его связи с тегами
func (r *Repository) CreateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, insertDishQuery,
		d.Name, d.Description, d.Composition, d.ImageURL,
		d.PriceMinor, d.Currency,
		d.CaloriesKcal, d.ProteinG, d.FatG, d.CarbsG, d.PortionWeightG,
		d.Cuisine, d.CategoryID,
		d.Allergens, d.Dietary,
		d.IsAvailable,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "dishes_name_key") {
			return menu.ErrDishNameTaken
		}
		return fmt.Errorf("insert dish: %w", err)
	}

	if linkErr := insertDishTags(ctx, tx, d.ID, tagIDs); linkErr != nil {
		return linkErr
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("commit: %w", commitErr)
	}

	tags, err := r.tagsForDish(ctx, d.ID)
	if err != nil {
		return err
	}
	d.Tags = tags
	return nil
}

// UpdateDish обновляет блюдо и (если tagIDs != nil) перепривязывает теги
func (r *Repository) UpdateDish(ctx context.Context, d *repositorymodels.Dish, tagIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cmd, err := tx.Exec(ctx, updateDishQuery,
		d.ID,
		d.Name, d.Description, d.Composition, d.ImageURL,
		d.PriceMinor, d.Currency,
		d.CaloriesKcal, d.ProteinG, d.FatG, d.CarbsG, d.PortionWeightG,
		d.Cuisine, d.CategoryID,
		d.Allergens, d.Dietary,
		d.IsAvailable,
	)
	if err != nil {
		if isUniqueViolation(err, "dishes_name_key") {
			return menu.ErrDishNameTaken
		}
		return fmt.Errorf("update dish: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrDishNotFound
	}

	if tagIDs != nil {
		if _, err := tx.Exec(ctx, deleteDishTagsQuery, d.ID); err != nil {
			return fmt.Errorf("clear dish tags: %w", err)
		}
		if err := insertDishTags(ctx, tx, d.ID, tagIDs); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// SetDishAvailability обновляет is_available
func (r *Repository) SetDishAvailability(ctx context.Context, id int, available bool) error {
	cmd, err := r.pool.Exec(ctx, setDishAvailabilityQuery, id, available)
	if err != nil {
		return fmt.Errorf("set availability: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return menu.ErrDishNotFound
	}
	return nil
}

func insertDishTags(ctx context.Context, tx pgx.Tx, dishID int, tagIDs []int) error {
	for _, tID := range tagIDs {
		if _, err := tx.Exec(ctx, insertDishTagQuery, dishID, tID); err != nil {
			return fmt.Errorf("insert dish_tag: %w", err)
		}
	}
	return nil
}

func (r *Repository) tagsForDish(ctx context.Context, dishID int) ([]repositorymodels.Tag, error) {
	const q = `
		SELECT ` + tagColumnsPrefixed + `
		FROM tags
		JOIN dish_tags ON dish_tags.tag_id = tags.id
		WHERE dish_tags.dish_id = $1
		ORDER BY tags.name`
	rows, err := r.pool.Query(ctx, q, dishID)
	if err != nil {
		return nil, fmt.Errorf("query tags for dish: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r *Repository) attachTags(ctx context.Context, dishes []repositorymodels.Dish, dishIDs []int) error {
	if len(dishIDs) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx, dishTagsForDishesQuery, dishIDs)
	if err != nil {
		return fmt.Errorf("query dish tags: %w", err)
	}
	defer rows.Close()

	tagsByDish := map[int][]repositorymodels.Tag{}
	for rows.Next() {
		var dishID int
		var t repositorymodels.Tag
		if err := rows.Scan(&dishID, &t.ID, &t.Name, &t.Slug, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return fmt.Errorf("scan dish_tag: %w", err)
		}
		tagsByDish[dishID] = append(tagsByDish[dishID], t)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows dish_tags: %w", err)
	}
	for i := range dishes {
		dishes[i].Tags = tagsByDish[dishes[i].ID]
	}
	return nil
}

func scanDish(row pgx.Row) (*repositorymodels.Dish, error) {
	var d repositorymodels.Dish
	err := row.Scan(
		&d.ID,
		&d.Name,
		&d.Description,
		&d.Composition,
		&d.ImageURL,
		&d.PriceMinor,
		&d.Currency,
		&d.CaloriesKcal,
		&d.ProteinG,
		&d.FatG,
		&d.CarbsG,
		&d.PortionWeightG,
		&d.Cuisine,
		&d.CategoryID,
		&d.Allergens,
		&d.Dietary,
		&d.IsAvailable,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if d.Allergens == nil {
		d.Allergens = []string{}
	}
	if d.Dietary == nil {
		d.Dietary = []string{}
	}
	return &d, nil
}

// buildDishWhere собирает WHERE-кляузу с параметризованными аргументами
func buildDishWhere(f repositorymodels.DishFilter) (string, []any) {
	var conds []string
	var args []any

	add := func(cond string, vals ...any) {
		offset := len(args)
		for i := range vals {
			cond = strings.Replace(cond, "?", "$"+strconv.Itoa(offset+i+1), 1)
		}
		conds = append(conds, cond)
		args = append(args, vals...)
	}

	if f.CategoryID != nil {
		add("category_id = ?", *f.CategoryID)
	}
	if f.Available != nil {
		add("is_available = ?", *f.Available)
	} else {
		// по умолчанию public-выдача — только доступные
		add("is_available = ?", true)
	}
	if f.Q != "" {
		add("name ILIKE ?", "%"+f.Q+"%")
	}
	if len(f.ExcludeAllergens) > 0 {
		add("NOT (allergens && ?)", f.ExcludeAllergens)
	}
	if len(f.Dietary) > 0 {
		add("dietary @> ?", f.Dietary)
	}
	if len(f.TagIDs) > 0 {
		add("id IN (SELECT dish_id FROM dish_tags WHERE tag_id = ANY(?))", f.TagIDs)
	}

	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}
