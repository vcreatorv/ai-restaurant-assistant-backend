// Package audit пишет и читает лог админских действий.
//
// Запись делает Recorder из usecase-слоя любой фичи (orders, menu, prompts):
//
//	audit.Recorder.Record(ctx, audit.Entry{...})
//
// Чтение — через Reader (для GET /admin/actions, /admin/orders/{id}/actions).
//
// Запись делается best-effort и НЕ должна валить основной usecase: Record
// логирует ошибку и возвращает nil, чтобы аудит не блокировал бизнес-операцию.
package audit

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidFilter переданный фильтр некорректен (например, плохой uuid в admin_id).
var ErrInvalidFilter = errors.New("invalid audit filter")

// Target тип объекта, на котором было совершено действие.
type Target string

const (
	// TargetOrder заказ.
	TargetOrder Target = "order"
	// TargetDish блюдо.
	TargetDish Target = "dish"
	// TargetCategory категория меню.
	TargetCategory Target = "category"
	// TargetTag тег.
	TargetTag Target = "tag"
	// TargetPrompt системный промпт LLM.
	TargetPrompt Target = "prompt"
)

// Verb что именно сделали.
type Verb string

const (
	// VerbCreate создание сущности.
	VerbCreate Verb = "create"
	// VerbUpdate частичное обновление.
	VerbUpdate Verb = "update"
	// VerbDelete удаление (soft или hard).
	VerbDelete Verb = "delete"
	// VerbStatusChange смена статуса заказа.
	VerbStatusChange Verb = "status_change"
	// VerbPublish публикация новой версии промпта.
	VerbPublish Verb = "publish"
	// VerbRollback откат промпта к старой версии.
	VerbRollback Verb = "rollback"
)

// Change один пункт диффа: что поменялось.
// Both from и to — текстовое представление; коды/числа конвертируются в фронте.
type Change struct {
	// Field имя поля (status, Цена, Доступно, Версия и т.п.)
	Field string
	// From исходное значение (опционально для create/delete)
	From string
	// To новое значение (опционально для delete)
	To string
}

// Entry заполняемые usecase'ом поля. AdminID и CreatedAt репозиторий проставит сам.
type Entry struct {
	// AdminID id админа, выполнившего действие
	AdminID uuid.UUID
	// Target тип объекта
	Target Target
	// TargetID идентификатор объекта (uuid/int/string — записываем как есть)
	TargetID string
	// TargetLabel «человеческое» имя на момент действия (#1024, Том Ям, system_main)
	TargetLabel string
	// Verb что сделали
	Verb Verb
	// Changes дифф (опционально)
	Changes []Change
}

// Author автор действия в выдаче. Пустые поля — если автор удалён (admin_id IS NULL).
type Author struct {
	// ID id админа (nil если удалён)
	ID *uuid.UUID
	// DisplayName собранное «Имя Фамилия» либо email
	DisplayName string
	// Email почта (nil если автор удалён)
	Email *string
	// HasNamesake есть ли в БД ещё один админ с таким же ФИО
	HasNamesake bool
}

// Action готовая запись аудит-лога для выдачи.
type Action struct {
	// ID stable-id записи (bigserial → string чтобы не светить раздачу id-ов)
	ID string
	// Admin автор
	Admin Author
	// Target тип объекта
	Target Target
	// TargetID идентификатор
	TargetID string
	// TargetLabel «человеческое» имя
	TargetLabel string
	// Verb что сделали
	Verb Verb
	// Changes дифф
	Changes []Change
	// CreatedAt когда
	CreatedAt time.Time
}

// Filter параметры выборки для GET /admin/actions.
// Если AdminID/Target/From/To нулевые — фильтр не применяется.
type Filter struct {
	// AdminID если задан — выборка только по этому админу
	AdminID *uuid.UUID
	// Target если задан — фильтр по типу
	Target *Target
	// From created_at >= From
	From *time.Time
	// To created_at <= To
	To *time.Time
	// Limit пагинация (1..200)
	Limit int
	// Offset пагинация
	Offset int
}

// Recorder пишет действие в лог. Best-effort: ошибка возвращается, но
// usecase-слой обычно её только логирует, не валя бизнес-операцию.
type Recorder interface {
	Record(ctx context.Context, e Entry) error
}

// Reader выборка из лога.
type Reader interface {
	List(ctx context.Context, f Filter) ([]Action, int, error)
	ListByOrder(ctx context.Context, orderID uuid.UUID, limit, offset int) ([]Action, int, error)
}

// Repository хранилище аудит-лога.
type Repository interface {
	Recorder
	Reader
}
