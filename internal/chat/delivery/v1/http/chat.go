package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"

	"github.com/google/uuid"
)

// GetActiveChat реализует GET /chats/active
func (h ChatHandler) GetActiveChat(
	ctx context.Context,
	_ v1.GetActiveChatRequestObject,
) (v1.GetActiveChatResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.usecase.GetActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	return v1.GetActiveChat200JSONResponse(apimodels.ChatFromUsecase(c)), nil
}

// ListChats реализует GET /chats
func (h ChatHandler) ListChats(
	ctx context.Context,
	request v1.ListChatsRequestObject,
) (v1.ListChatsResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := h.resolveListPaging(request.Params.Limit, request.Params.Offset)
	items, total, err := h.usecase.List(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return v1.ListChats200JSONResponse(apimodels.ChatListFromUsecase(items, total, limit, offset)), nil
}

// CreateChat реализует POST /chats
func (h ChatHandler) CreateChat(
	ctx context.Context,
	request v1.CreateChatRequestObject,
) (v1.CreateChatResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	var title *string
	if request.Body != nil {
		title = request.Body.Title
	}
	c, err := h.usecase.Create(ctx, userID, title)
	if err != nil {
		return nil, err
	}
	return v1.CreateChat201JSONResponse(apimodels.ChatFromUsecase(c)), nil
}

// GetChat реализует GET /chats/{id}
func (h ChatHandler) GetChat(
	ctx context.Context,
	request v1.GetChatRequestObject,
) (v1.GetChatResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit := h.resolveMessagesLimit(request.Params.MessagesLimit)
	var before *uuid.UUID
	if request.Params.MessagesBefore != nil {
		v := *request.Params.MessagesBefore
		before = &v
	}
	c, msgs, hasMore, err := h.usecase.GetWithMessages(ctx, userID, request.Id, limit, before)
	if err != nil {
		return nil, err
	}
	return v1.GetChat200JSONResponse(apimodels.ChatWithMessagesFromUsecase(c, msgs, hasMore)), nil
}

// DeleteChat реализует DELETE /chats/{id}
func (h ChatHandler) DeleteChat(
	ctx context.Context,
	request v1.DeleteChatRequestObject,
) (v1.DeleteChatResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.Delete(ctx, userID, request.Id); err != nil {
		return nil, err
	}
	return v1.DeleteChat204Response{}, nil
}

// SendMessage реализует POST /chats/{id}/messages.
// io.Pipe позволяет стримить SSE через strict-server: usecase в горутине пишет
// события в pw, io.Copy из стрикта читает из pr и отдаёт клиенту чанками.
func (h ChatHandler) SendMessage(
	ctx context.Context,
	request v1.SendMessageRequestObject,
) (v1.SendMessageResponseObject, error) {
	userID, err := h.requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil || request.Body.Content == "" {
		return nil, apperrors.ErrBadRequest
	}

	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		runErr := h.usecase.SendMessage(ctx, userID, request.Id, request.Body.Content, chat.SendCallbacks{
			OnMeta: func(m chat.MetaEvent) error {
				return writeSSEEvent(pw, "meta", map[string]any{
					"message_id":           m.MessageID,
					"recommended_dish_ids": coalesceInts(m.RecommendedDishIDs),
				})
			},
			OnDelta: func(delta string) error {
				return writeSSEEvent(pw, "token", map[string]any{"delta": delta})
			},
			OnDone: func(d chat.DoneEvent) error {
				return writeSSEEvent(pw, "done", map[string]any{
					"latency_ms": d.LatencyMS,
					"tokens_in":  d.TokensIn,
					"tokens_out": d.TokensOut,
					"model":      d.Model,
				})
			},
		})
		if runErr != nil {
			logger.ForCtx(ctx).Error("send message", "err", runErr)
			_ = writeSSEEvent(pw, "error", map[string]any{
				"code":    "upstream_failure",
				"message": "Failed to generate assistant response",
			})
		}
	}()

	return v1.SendMessage200TexteventStreamResponse{Body: pr}, nil
}

// requireUserID возвращает userID из контекстной сессии или ErrUnauthenticated
func (h ChatHandler) requireUserID(ctx context.Context) (uuid.UUID, error) {
	s := middleware.SessionFromCtx(ctx)
	if s == nil || s.UserID == nil {
		return uuid.Nil, apperrors.ErrUnauthenticated
	}
	return *s.UserID, nil
}

// resolveListPaging применяет дефолты и кэп для list-эндпойнта чатов
func (h ChatHandler) resolveListPaging(limit, offset *int) (int, int) {
	l := h.cfg.ListDefaultLimit
	if limit != nil {
		l = *limit
	}
	if l <= 0 {
		l = h.cfg.ListDefaultLimit
	}
	if h.cfg.ListMaxLimit > 0 && l > h.cfg.ListMaxLimit {
		l = h.cfg.ListMaxLimit
	}
	o := 0
	if offset != nil && *offset > 0 {
		o = *offset
	}
	return l, o
}

// resolveMessagesLimit применяет дефолт и кэп для history
func (h ChatHandler) resolveMessagesLimit(limit *int) int {
	l := h.cfg.MessagesDefaultLimit
	if limit != nil && *limit > 0 {
		l = *limit
	}
	if h.cfg.MessagesMaxLimit > 0 && l > h.cfg.MessagesMaxLimit {
		l = h.cfg.MessagesMaxLimit
	}
	return l
}

// writeSSEEvent сериализует payload в JSON и пишет SSE-event в writer
func writeSSEEvent(w io.Writer, event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{}`)
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		return err
	}
	return nil
}

func coalesceInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
