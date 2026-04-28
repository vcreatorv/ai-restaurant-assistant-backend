package repository

import (
	"time"

	"github.com/google/uuid"
)

// User пользователь в storage-форме
type User struct {
	// ID идентификатор пользователя
	ID uuid.UUID
	// Email email
	Email *string
	// PasswordHash хэш пароля
	PasswordHash *string
	// Role роль: guest, customer, admin
	Role string
	// FirstName имя
	FirstName *string
	// LastName фамилия
	LastName *string
	// Phone телефон в формате E.164
	Phone *string
	// Allergens коды аллергенов
	Allergens []string
	// Dietary коды диетических предпочтений
	Dietary []string
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}
