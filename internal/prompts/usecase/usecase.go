// Package usecase реализует prompts.Usecase.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	"github.com/google/uuid"
)

const (
	// MinContentLen синхронизирован с CHECK в prompts.up.sql и с фронтом.
	MinContentLen = 50
	// MaxContentLen синхронизирован с CHECK в prompts.up.sql и с фронтом.
	MaxContentLen = 8000
)

// requiredPlaceholders плейсхолдеры, обязательные в каждом name.
// Без них промпт не сможет получить контекст и поведение сломается.
var requiredPlaceholders = map[prompts.Name][]string{
	// system — контекст блюд приходит в user-сообщении, не в system; placeholder'ов нет.
	prompts.NameSystem: nil,
	// classification и refusal принимают user_message прямо в тело промпта —
	// usecase подставляет {{user_message}} перед отправкой в LLM.
	prompts.NameClassification: {"{{user_message}}"},
	prompts.NameRefusal:        {"{{user_message}}"},
}

type usecaseImpl struct {
	repo prompts.Repository
}

// New собирает Usecase.
func New(repo prompts.Repository) prompts.Usecase {
	return &usecaseImpl{repo: repo}
}

// List возвращает список промптов с активной версией и (если есть) черновиком текущего админа.
func (u *usecaseImpl) List(ctx context.Context, adminID uuid.UUID) ([]prompts.Prompt, error) {
	out := make([]prompts.Prompt, 0, len(prompts.SupportedNames))
	for _, name := range prompts.SupportedNames {
		v, err := u.repo.FindActiveVersion(ctx, name)
		if err != nil {
			if errors.Is(err, prompts.ErrNotFound) {
				continue
			}
			return nil, err
		}
		draft, err := u.repo.FindDraft(ctx, adminID, name)
		if err != nil {
			return nil, err
		}
		out = append(out, prompts.Prompt{Name: name, Current: *v, Draft: draft})
	}
	return out, nil
}

// Get возвращает промпт целиком: активная версия + мой черновик + история.
func (u *usecaseImpl) Get(ctx context.Context, name prompts.Name, adminID uuid.UUID) (*prompts.Details, error) {
	if !prompts.IsSupported(name) {
		return nil, prompts.ErrUnknownName
	}
	v, err := u.repo.FindActiveVersion(ctx, name)
	if err != nil {
		return nil, err
	}
	history, err := u.repo.ListVersions(ctx, name)
	if err != nil {
		return nil, err
	}
	draft, err := u.repo.FindDraft(ctx, adminID, name)
	if err != nil {
		return nil, err
	}
	return &prompts.Details{Name: name, Current: *v, Draft: draft, History: history}, nil
}

// UpsertDraft сохраняет/обновляет черновик. Перед записью валидирует content.
func (u *usecaseImpl) UpsertDraft(
	ctx context.Context,
	name prompts.Name,
	adminID uuid.UUID,
	content string,
) (*prompts.Draft, error) {
	if !prompts.IsSupported(name) {
		return nil, prompts.ErrUnknownName
	}
	if err := validateContent(name, content); err != nil {
		return nil, err
	}
	return u.repo.UpsertDraft(ctx, adminID, name, content)
}

// DeleteDraft удаляет мой черновик (idempotent).
func (u *usecaseImpl) DeleteDraft(ctx context.Context, name prompts.Name, adminID uuid.UUID) error {
	if !prompts.IsSupported(name) {
		return prompts.ErrUnknownName
	}
	return u.repo.DeleteDraft(ctx, adminID, name)
}

// Publish публикует мой черновик как новую версию.
// Если черновика нет — ErrDraftNotFound.
func (u *usecaseImpl) Publish(ctx context.Context, name prompts.Name, adminID uuid.UUID) (*prompts.Version, error) {
	if !prompts.IsSupported(name) {
		return nil, prompts.ErrUnknownName
	}
	draft, err := u.repo.FindDraft(ctx, adminID, name)
	if err != nil {
		return nil, err
	}
	if draft == nil {
		return nil, prompts.ErrDraftNotFound
	}
	if err := validateContent(name, draft.Content); err != nil {
		return nil, err
	}
	return u.repo.InsertVersion(ctx, name, draft.Content, adminID, true)
}

// Rollback создаёт новую запись с content из старой версии.
func (u *usecaseImpl) Rollback(
	ctx context.Context,
	name prompts.Name,
	version int,
	adminID uuid.UUID,
) (*prompts.Version, error) {
	if !prompts.IsSupported(name) {
		return nil, prompts.ErrUnknownName
	}
	src, err := u.repo.FindVersion(ctx, name, version)
	if err != nil {
		return nil, err
	}
	// Откат не трогает personal draft — оставляем как есть.
	return u.repo.InsertVersion(ctx, name, src.Content, adminID, false)
}

// GetActive возвращает контент промпта для подстановки в LLM-запрос.
// Если authorID != Nil и под ним есть черновик — возвращается черновик.
func (u *usecaseImpl) GetActive(ctx context.Context, name prompts.Name, authorID uuid.UUID) (string, error) {
	if !prompts.IsSupported(name) {
		return "", prompts.ErrUnknownName
	}
	if authorID != uuid.Nil {
		content, ok, err := u.repo.FindDraftContent(ctx, authorID, name)
		if err != nil {
			return "", err
		}
		if ok {
			return content, nil
		}
	}
	return u.repo.FindActiveContent(ctx, name)
}

// validateContent проверяет длину и обязательные плейсхолдеры.
//
// Лимит сравнивается по СИМВОЛАМ (utf8.RuneCountInString), а не по байтам:
// БД-CHECK на колонке content использует Postgres length() — тоже посимвольный счёт.
// Если считать через len() (байты), для кириллицы 1 символ = 2 байта, и промпт
// в 5500 символов отбивается как «11000 > MaxContentLen», хотя БД пропустила бы.
func validateContent(name prompts.Name, content string) error {
	if l := utf8.RuneCountInString(content); l < MinContentLen || l > MaxContentLen {
		return fmt.Errorf("%w: length %d not in [%d..%d]",
			prompts.ErrInvalidContent, l, MinContentLen, MaxContentLen)
	}
	missing := []string{}
	for _, ph := range requiredPlaceholders[name] {
		if !strings.Contains(content, ph) {
			missing = append(missing, ph)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing placeholders %s",
			prompts.ErrInvalidContent, strings.Join(missing, ", "))
	}
	return nil
}
