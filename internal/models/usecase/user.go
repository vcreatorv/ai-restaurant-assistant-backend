package usecase

import (
	"time"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
)

// Role роль пользователя
type Role string

const (
	// RoleGuest анонимный пользователь
	RoleGuest Role = "guest"
	// RoleCustomer зарегистрированный пользователь
	RoleCustomer Role = "customer"
	// RoleAdmin администратор
	RoleAdmin Role = "admin"
)

// User пользователь в доменной форме
type User struct {
	// ID идентификатор пользователя
	ID uuid.UUID
	// Email email
	Email string
	// PasswordHash хэш пароля
	PasswordHash string
	// Role роль
	Role Role
	// FirstName имя
	FirstName string
	// LastName фамилия
	LastName string
	// Phone телефон в формате E.164
	Phone string
	// Allergens коды аллергенов
	Allergens []string
	// Dietary коды диетических предпочтений
	Dietary []string
	// CreatedAt время создания
	CreatedAt time.Time
	// UpdatedAt время последнего обновления
	UpdatedAt time.Time
}

// IsGuest проверяет, что пользователь — гость
func (u *User) IsGuest() bool { return u.Role == RoleGuest }

// IsCustomer проверяет, что пользователь — customer
func (u *User) IsCustomer() bool { return u.Role == RoleCustomer }

// IsAdmin проверяет, что пользователь — админ
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// ProfilePatch частичное обновление профиля
type ProfilePatch struct {
	// FirstName имя
	FirstName *string
	// LastName фамилия
	LastName *string
	// Phone телефон
	Phone *string
	// Allergens коды аллергенов
	Allergens *[]string
	// Dietary коды диетических предпочтений
	Dietary *[]string
}

// UserFromRepository маппит repository-модель в usecase-модель
func UserFromRepository(r *repositorymodels.User) *User {
	if r == nil {
		return nil
	}
	return &User{
		ID:           r.ID,
		Email:        derefString(r.Email),
		PasswordHash: derefString(r.PasswordHash),
		Role:         Role(r.Role),
		FirstName:    derefString(r.FirstName),
		LastName:     derefString(r.LastName),
		Phone:        derefString(r.Phone),
		Allergens:    coalesceSlice(r.Allergens),
		Dietary:      coalesceSlice(r.Dietary),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

// UserToRepository маппит usecase-модель в repository-модель
func UserToRepository(u *User) *repositorymodels.User {
	return &repositorymodels.User{
		ID:           u.ID,
		Email:        nullableString(u.Email),
		PasswordHash: nullableString(u.PasswordHash),
		Role:         string(u.Role),
		FirstName:    nullableString(u.FirstName),
		LastName:     nullableString(u.LastName),
		Phone:        nullableString(u.Phone),
		Allergens:    u.Allergens,
		Dietary:      u.Dietary,
	}
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func coalesceSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
