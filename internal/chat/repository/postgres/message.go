package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const messageColumns = `id, chat_id, role, content, recommended_dish_ids, meta, created_at`

const (
	insertMessageQuery = `
		INSERT INTO chat_messages (id, chat_id, role, content, recommended_dish_ids, meta)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`

	touchChatQuery = `
		UPDATE chats
		SET last_message_at = now(),
		    updated_at = now()
		WHERE id = $1`

	listMessagesQuery = `
		SELECT ` + messageColumns + `
		FROM chat_messages
		WHERE chat_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`

	listMessagesBeforeQuery = `
		SELECT ` + messageColumns + `
		FROM chat_messages
		WHERE chat_id = $1
		  AND created_at < (SELECT created_at FROM chat_messages WHERE id = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3`
)

// AppendMessage сохраняет сообщение и обновляет last_message_at чата в одной транзакции
func (r *Repository) AppendMessage(ctx context.Context, m *repositorymodels.Message) error {
	meta := m.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	dishIDs := m.RecommendedDishIDs
	if dishIDs == nil {
		dishIDs = []int{}
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, insertMessageQuery,
		m.ID, m.ChatID, m.Role, m.Content, dishIDs, metaJSON,
	).Scan(&m.CreatedAt); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	if _, err := tx.Exec(ctx, touchChatQuery, m.ChatID); err != nil {
		return fmt.Errorf("touch chat: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ListMessages возвращает сообщения чата от новых к старым; before — id сообщения-курсора
func (r *Repository) ListMessages(
	ctx context.Context,
	chatID uuid.UUID,
	limit int,
	before *uuid.UUID,
) ([]repositorymodels.Message, bool, error) {
	// Запрашиваем на 1 больше, чтобы определить has_more.
	fetch := limit + 1

	var (
		rows pgx.Rows
		err  error
	)
	if before == nil {
		rows, err = r.pool.Query(ctx, listMessagesQuery, chatID, fetch)
	} else {
		rows, err = r.pool.Query(ctx, listMessagesBeforeQuery, chatID, *before, fetch)
	}
	if err != nil {
		return nil, false, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	out := make([]repositorymodels.Message, 0, fetch)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("rows messages: %w", err)
	}

	hasMore := false
	if len(out) > limit {
		hasMore = true
		out = out[:limit]
	}
	return out, hasMore, nil
}

func scanMessage(row pgx.Row) (*repositorymodels.Message, error) {
	var (
		m       repositorymodels.Message
		metaRaw []byte
	)
	if err := row.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.RecommendedDishIDs, &metaRaw, &m.CreatedAt); err != nil {
		return nil, err
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &m.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}
	return &m, nil
}
