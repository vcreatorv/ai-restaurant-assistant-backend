// Package prompts управляет версионируемыми системными промптами LLM.
//
// Чат на каждом запросе берёт активную версию через Usecase.GetActive(name);
// если автор сообщения — админ и под ним есть личный черновик, usecase
// возвращает контент черновика (отладочный режим). При publish создаётся
// новая запись prompts с version+1, draft удаляется.
package prompts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrUnknownName переданное name не входит в SupportedNames.
	ErrUnknownName = errors.New("unknown prompt name")
	// ErrNotFound по name не нашлось ни одной версии.
	ErrNotFound = errors.New("prompt not found")
	// ErrDraftNotFound у админа нет черновика по запрошенному name.
	ErrDraftNotFound = errors.New("prompt draft not found")
	// ErrInvalidContent контент не прошёл валидацию.
	ErrInvalidContent = errors.New("prompt content is invalid")
	// ErrVersionNotFound запрошенная версия не существует (для rollback).
	ErrVersionNotFound = errors.New("prompt version not found")
)

// Name именованный промпт. Расширяем по мере появления отдельных промптов.
type Name string

const (
	// NameSystem основной системный промпт чата (рекомендации с RAG-контекстом).
	NameSystem Name = "system"
	// NameClassification классификатор намерения пользователя
	// (recommend / clarify / chitchat / off_topic).
	NameClassification Name = "classification"
	// NameRefusal короткий вежливый отказ для off-topic запросов.
	NameRefusal Name = "refusal"
)

// SupportedNames список разрешённых имён. Сверяемся с ним в usecase до похода в БД —
// единственный источник истины (вне зависимости от OpenAPI). Чтобы добавить новый
// промпт, достаточно дописать константу выше + сюда.
var SupportedNames = []Name{NameSystem, NameClassification, NameRefusal}

// IsSupported проверяет, что имя из enum.
func IsSupported(n Name) bool {
	for _, s := range SupportedNames {
		if s == n {
			return true
		}
	}
	return false
}

// Author минимальные данные об авторе версии (для отображения в админке).
type Author struct {
	// ID идентификатор пользователя в users
	ID uuid.UUID
	// DisplayName собранное «Имя Фамилия»
	DisplayName string
	// Email почта (используется для дисамбигуации тёзок)
	Email string
}

// Version опубликованная версия промпта.
type Version struct {
	// Version номер версии в рамках одного name (возрастает)
	Version int
	// Content содержимое промпта
	Content string
	// PublishedAt когда опубликована
	PublishedAt time.Time
	// PublishedBy кто опубликовал
	PublishedBy Author
}

// Draft личный черновик админа на промпт.
type Draft struct {
	// Name имя промпта
	Name Name
	// Content содержимое черновика
	Content string
	// UpdatedAt последнее изменение
	UpdatedAt time.Time
}

// Prompt краткое представление: name + активная версия + (опционально) мой черновик.
type Prompt struct {
	// Name имя промпта
	Name Name
	// Current активная (последняя по version) запись
	Current Version
	// Draft мой черновик, если есть
	Draft *Draft
}

// Details подробности по промпту: краткое + полная история версий.
type Details struct {
	// Name имя промпта
	Name Name
	// Current активная версия
	Current Version
	// Draft мой черновик, если есть
	Draft *Draft
	// History все версии (включая current) по убыванию Version
	History []Version
}

// Usecase сценарии работы с промптами.
type Usecase interface {
	// List возвращает список промптов с активной версией и (если есть) черновиком текущего админа.
	List(ctx context.Context, adminID uuid.UUID) ([]Prompt, error)
	// Get возвращает промпт целиком: активная версия + мой черновик + история.
	Get(ctx context.Context, name Name, adminID uuid.UUID) (*Details, error)
	// UpsertDraft сохраняет/обновляет черновик админа. Валидирует content.
	UpsertDraft(ctx context.Context, name Name, adminID uuid.UUID, content string) (*Draft, error)
	// DeleteDraft удаляет мой черновик (idempotent).
	DeleteDraft(ctx context.Context, name Name, adminID uuid.UUID) error
	// Publish публикует мой черновик как новую версию. Удаляет черновик в той же транзакции.
	Publish(ctx context.Context, name Name, adminID uuid.UUID) (*Version, error)
	// Rollback откатывает к указанной старой версии: создаёт новую запись с её content.
	Rollback(ctx context.Context, name Name, version int, adminID uuid.UUID) (*Version, error)
	// GetActive возвращает контент промпта для подстановки в LLM-запрос.
	// Если authorID != uuid.Nil и под ним есть черновик — берётся черновик, иначе активная версия.
	GetActive(ctx context.Context, name Name, authorID uuid.UUID) (string, error)
}

// Repository хранилище промптов.
type Repository interface {
	// FindActiveVersion возвращает запись с максимальным version по name.
	FindActiveVersion(ctx context.Context, name Name) (*Version, error)
	// FindActiveContent — оптимизированная выборка только content (без author/timestamps).
	FindActiveContent(ctx context.Context, name Name) (string, error)
	// ListVersions возвращает все версии по name по убыванию version.
	ListVersions(ctx context.Context, name Name) ([]Version, error)
	// FindVersion возвращает конкретную версию.
	FindVersion(ctx context.Context, name Name, version int) (*Version, error)
	// InsertVersion создаёт новую запись с version = max+1 и удаляет draft в одной транзакции.
	// Возвращает созданную запись с заполненными PublishedAt/PublishedBy.
	InsertVersion(ctx context.Context, name Name, content string, authorID uuid.UUID, deleteDraft bool) (*Version, error)

	// FindDraft возвращает черновик админа (или nil если нет).
	FindDraft(ctx context.Context, adminID uuid.UUID, name Name) (*Draft, error)
	// FindDraftContent — оптимизированная выборка только content.
	FindDraftContent(ctx context.Context, adminID uuid.UUID, name Name) (string, bool, error)
	// UpsertDraft создаёт или обновляет черновик.
	UpsertDraft(ctx context.Context, adminID uuid.UUID, name Name, content string) (*Draft, error)
	// DeleteDraft удаляет черновик (idempotent).
	DeleteDraft(ctx context.Context, adminID uuid.UUID, name Name) error
}
