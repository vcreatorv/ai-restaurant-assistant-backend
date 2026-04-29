package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const chatColumns = `id, user_id, title, last_message_at, created_at, updated_at`

const (
	insertChatQuery = `
		INSERT INTO chats (id, user_id, title)
		VALUES ($1, $2, $3)
		RETURNING last_message_at, created_at, updated_at`

	findChatByIDQuery = `
		SELECT ` + chatColumns + `
		FROM chats
		WHERE id = $1`

	findMostRecentChatQuery = `
		SELECT ` + chatColumns + `
		FROM chats
		WHERE user_id = $1
		ORDER BY last_message_at DESC
		LIMIT 1`

	listChatsByUserQuery = `
		SELECT ` + chatColumns + `
		FROM chats
		WHERE user_id = $1
		ORDER BY last_message_at DESC
		LIMIT $2 OFFSET $3`

	countChatsByUserQuery = `SELECT COUNT(*) FROM chats WHERE user_id = $1`

	deleteChatQuery = `DELETE FROM chats WHERE id = $1`
)

// CreateChat вставляет чат в БД
func (r *Repository) CreateChat(ctx context.Context, c *repositorymodels.Chat) error {
	err := r.pool.QueryRow(ctx, insertChatQuery, c.ID, c.UserID, c.Title).
		Scan(&c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert chat: %w", err)
	}
	return nil
}

// FindChatByID возвращает чат по идентификатору
func (r *Repository) FindChatByID(ctx context.Context, id uuid.UUID) (*repositorymodels.Chat, error) {
	row := r.pool.QueryRow(ctx, findChatByIDQuery, id)
	c, err := scanChat(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, chat.ErrChatNotFound
	}
	return c, err
}

// FindMostRecentByUser возвращает самый свежий чат пользователя
func (r *Repository) FindMostRecentByUser(ctx context.Context, userID uuid.UUID) (*repositorymodels.Chat, error) {
	row := r.pool.QueryRow(ctx, findMostRecentChatQuery, userID)
	c, err := scanChat(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, chat.ErrChatNotFound
	}
	return c, err
}

// ListChatsByUser возвращает чаты пользователя
func (r *Repository) ListChatsByUser(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]repositorymodels.Chat, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countChatsByUserQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count chats: %w", err)
	}
	rows, err := r.pool.Query(ctx, listChatsByUserQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query chats: %w", err)
	}
	defer rows.Close()

	out := make([]repositorymodels.Chat, 0, limit)
	for rows.Next() {
		c, err := scanChat(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows chats: %w", err)
	}
	return out, total, nil
}

// DeleteChat удаляет чат
func (r *Repository) DeleteChat(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, deleteChatQuery, id)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return chat.ErrChatNotFound
	}
	return nil
}

func scanChat(row pgx.Row) (*repositorymodels.Chat, error) {
	var c repositorymodels.Chat
	if err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}
