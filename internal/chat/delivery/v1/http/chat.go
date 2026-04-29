package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
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
// На A1 ассистент отвечает эхо-заглушкой; стрим SSE собирается синхронно в буфер и
// отдаётся клиенту целиком через io.Reader-механику strict-сервера.
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
	start := time.Now()
	_, assistantMsg, err := h.usecase.SendMessage(ctx, userID, request.Id, request.Body.Content)
	if err != nil {
		return nil, err
	}

	body := buildSSEStream(assistantMsg, time.Since(start))
	return v1.SendMessage200TexteventStreamResponse{
		Body:          body,
		ContentLength: int64(body.Len()),
	}, nil
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

// buildSSEStream собирает SSE-стрим (meta → token → done) в буфер.
// На A1 token отдаётся одним событием с полным echo-ответом; на A3 будет настоящий
// токен-стрим из LLM, который потребует обходить strict-сервер.
func buildSSEStream(assistantMsg *usecasemodels.Message, elapsed time.Duration) *bytes.Buffer {
	buf := &bytes.Buffer{}
	writeSSEEvent(buf, "meta", map[string]any{
		"message_id":           assistantMsg.ID,
		"recommended_dish_ids": coalesceInts(assistantMsg.RecommendedDishIDs),
	})
	writeSSEEvent(buf, "token", map[string]any{
		"delta": assistantMsg.Content,
	})
	writeSSEEvent(buf, "done", map[string]any{
		"tokens_in":  0,
		"tokens_out": 0,
		"latency_ms": elapsed.Milliseconds(),
	})
	return buf
}

func writeSSEEvent(buf *bytes.Buffer, event string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{}`)
	}
	fmt.Fprintf(buf, "event: %s\n", event)
	fmt.Fprintf(buf, "data: %s\n\n", raw)
}

func coalesceInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
