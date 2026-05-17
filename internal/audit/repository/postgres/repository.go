// Package postgres реализует audit.Repository (Recorder + Reader) поверх pgx.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository PostgreSQL-репозиторий аудита.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const (
	insertActionQuery = `
		INSERT INTO admin_actions (admin_id, target, target_id, target_label, verb, changes)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)`

	// listColumns — поля для выдачи Reader'ом.
	// Подтягиваем автора (LEFT JOIN: автор может быть удалён → admin_id=NULL)
	// и has_namesake одним SELECT, чтобы не делать N+1.
	//
	// has_namesake = true, если в users есть другой admin с такими же
	// (first_name, last_name) — регистронезависимо, исключая пустые.
	listColumns = `
		a.id, a.target, a.target_id, a.target_label, a.verb, a.changes, a.created_at,
		u.id AS admin_id,
		COALESCE(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.email) AS display_name,
		u.email AS email,
		CASE
		    WHEN u.id IS NULL THEN false
		    ELSE EXISTS (
		        SELECT 1 FROM users u2
		        WHERE u2.role = 'admin'
		          AND u2.id <> u.id
		          AND lower(COALESCE(u2.first_name, '')) = lower(COALESCE(u.first_name, ''))
		          AND lower(COALESCE(u2.last_name,  '')) = lower(COALESCE(u.last_name,  ''))
		          AND COALESCE(u2.first_name, '') <> ''
		          AND COALESCE(u2.last_name,  '') <> ''
		    )
		END AS has_namesake`

	listFromJoin = `
		FROM admin_actions a
		LEFT JOIN users u ON u.id = a.admin_id`
)

// Record вставляет запись в admin_actions. Если admin_id == uuid.Nil — пишем NULL.
func (r *Repository) Record(ctx context.Context, e audit.Entry) error {
	changesJSON, err := marshalChanges(e.Changes)
	if err != nil {
		return err
	}

	var adminID any
	if e.AdminID != uuid.Nil {
		adminID = e.AdminID
	}

	if _, err := r.pool.Exec(ctx, insertActionQuery,
		adminID,
		string(e.Target),
		e.TargetID,
		e.TargetLabel,
		string(e.Verb),
		changesJSON,
	); err != nil {
		return fmt.Errorf("insert admin_action: %w", err)
	}
	return nil
}

// List возвращает действия с фильтрами + total.
func (r *Repository) List(ctx context.Context, f audit.Filter) ([]audit.Action, int, error) {
	where, args := buildWhere(f)

	countQuery := "SELECT COUNT(*) " + listFromJoin + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count actions: %w", err)
	}

	limitArg := nextPlaceholder(len(args) + 1)
	offsetArg := nextPlaceholder(len(args) + 2)
	listQuery := "SELECT " + listColumns + " " + listFromJoin + where +
		" ORDER BY a.created_at DESC, a.id DESC LIMIT " + limitArg + " OFFSET " + offsetArg
	args = append(args, f.Limit, f.Offset)

	out, err := r.queryActions(ctx, listQuery, args, f.Limit)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListByOrder отдаёт историю изменений по конкретному заказу.
// Используется в GET /admin/orders/{id}/actions.
func (r *Repository) ListByOrder(
	ctx context.Context,
	orderID uuid.UUID,
	limit, offset int,
) ([]audit.Action, int, error) {
	const where = " WHERE a.target = $1 AND a.target_id = $2"

	countQuery := "SELECT COUNT(*) " + listFromJoin + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, string(audit.TargetOrder), orderID.String()).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count actions by order: %w", err)
	}

	listQuery := "SELECT " + listColumns + " " + listFromJoin + where +
		" ORDER BY a.created_at DESC, a.id DESC LIMIT $3 OFFSET $4"
	out, err := r.queryActions(ctx, listQuery,
		[]any{string(audit.TargetOrder), orderID.String(), limit, offset}, limit)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// queryActions общая выборка + сканирование. Total считает caller отдельным запросом.
func (r *Repository) queryActions(
	ctx context.Context,
	query string,
	args []any,
	expectedSize int,
) ([]audit.Action, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query actions: %w", err)
	}
	defer rows.Close()

	out := make([]audit.Action, 0, expectedSize)
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows actions: %w", err)
	}
	return out, nil
}

func buildWhere(f audit.Filter) (string, []any) {
	conds := []string{}
	args := []any{}

	if f.AdminID != nil {
		conds = append(conds, "a.admin_id = "+nextPlaceholder(len(args)+1))
		args = append(args, *f.AdminID)
	}
	if f.Target != nil {
		conds = append(conds, "a.target = "+nextPlaceholder(len(args)+1))
		args = append(args, string(*f.Target))
	}
	if f.From != nil {
		conds = append(conds, "a.created_at >= "+nextPlaceholder(len(args)+1))
		args = append(args, *f.From)
	}
	if f.To != nil {
		conds = append(conds, "a.created_at <= "+nextPlaceholder(len(args)+1))
		args = append(args, *f.To)
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func nextPlaceholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

// scanRow — общий интерфейс для pgx.Row / pgx.Rows.
type scanRow interface {
	Scan(dest ...any) error
}

func scanAction(row scanRow) (*audit.Action, error) {
	var (
		id          int64
		target      string
		targetID    string
		targetLabel string
		verb        string
		changesJSON []byte
		createdAt   time.Time
		adminUUID   pgtype.UUID
		displayName pgtype.Text
		email       pgtype.Text
		hasNamesake bool
	)

	if err := row.Scan(
		&id, &target, &targetID, &targetLabel, &verb, &changesJSON, &createdAt,
		&adminUUID, &displayName, &email, &hasNamesake,
	); err != nil {
		return nil, fmt.Errorf("scan action: %w", err)
	}

	a := &audit.Action{
		ID:          fmt.Sprintf("%d", id),
		Target:      audit.Target(target),
		TargetID:    targetID,
		TargetLabel: targetLabel,
		Verb:        audit.Verb(verb),
		CreatedAt:   createdAt,
	}
	if err := json.Unmarshal(changesJSON, &a.Changes); err != nil {
		// Не должно случаться (сами писали JSON), но не валим выдачу.
		a.Changes = nil
	}

	if adminUUID.Valid {
		uid := uuid.UUID(adminUUID.Bytes)
		a.Admin.ID = &uid
	}
	if displayName.Valid {
		a.Admin.DisplayName = displayName.String
	} else {
		a.Admin.DisplayName = "Удалённый админ"
	}
	if email.Valid {
		e := email.String
		a.Admin.Email = &e
	}
	a.Admin.HasNamesake = hasNamesake
	return a, nil
}

// marshalChanges возвращает каноническое JSON-представление списка изменений.
// Пустой список → "[]" (а не "null"), чтобы JSONB в БД был массивом.
func marshalChanges(changes []audit.Change) ([]byte, error) {
	if len(changes) == 0 {
		return []byte("[]"), nil
	}
	out := make([]map[string]string, len(changes))
	for i, c := range changes {
		m := map[string]string{"field": c.Field}
		if c.From != "" {
			m["from"] = c.From
		}
		if c.To != "" {
			m["to"] = c.To
		}
		out[i] = m
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal changes: %w", err)
	}
	return b, nil
}
