package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

const userColumns = `id, email, password_hash, role, first_name, last_name, phone, allergens, dietary, created_at, updated_at`

const (
	findByIDQuery    = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	findByEmailQuery = `SELECT ` + userColumns + ` FROM users WHERE email = $1`

	insertUserQuery = `
		INSERT INTO users (id, email, password_hash, role, first_name, last_name, phone, allergens, dietary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	updateUserQuery = `
		UPDATE users
		SET email = $2,
		    password_hash = $3,
		    role = $4,
		    first_name = $5,
		    last_name = $6,
		    phone = $7,
		    allergens = $8,
		    dietary = $9,
		    updated_at = now()
		WHERE id = $1`
)

// FindByID возвращает пользователя по идентификатору
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error) {
	row := r.pool.QueryRow(ctx, findByIDQuery, id)
	return scanUser(row)
}

// FindByEmail возвращает пользователя по email
func (r *Repository) FindByEmail(ctx context.Context, email string) (*repositorymodels.User, error) {
	if email == "" {
		return nil, user.ErrNotFound
	}
	row := r.pool.QueryRow(ctx, findByEmailQuery, email)
	return scanUser(row)
}

// Create вставляет нового пользователя
func (r *Repository) Create(ctx context.Context, u *repositorymodels.User) error {
	err := r.pool.QueryRow(
		ctx, insertUserQuery,
		u.ID, u.Email, u.PasswordHash, u.Role, u.FirstName, u.LastName, u.Phone, u.Allergens, u.Dietary,
	).Scan(&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "users_email_unique") {
			return user.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// Update сохраняет изменения пользователя
func (r *Repository) Update(ctx context.Context, u *repositorymodels.User) error {
	cmd, err := r.pool.Exec(
		ctx, updateUserQuery,
		u.ID, u.Email, u.PasswordHash, u.Role, u.FirstName, u.LastName, u.Phone, u.Allergens, u.Dietary,
	)
	if err != nil {
		if isUniqueViolation(err, "users_email_unique") {
			return user.ErrEmailTaken
		}
		return fmt.Errorf("update user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return user.ErrNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (*repositorymodels.User, error) {
	var u repositorymodels.User
	var role string
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&role,
		&u.FirstName,
		&u.LastName,
		&u.Phone,
		&u.Allergens,
		&u.Dietary,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.Role = role
	if u.Allergens == nil {
		u.Allergens = []string{}
	}
	if u.Dietary == nil {
		u.Dietary = []string{}
	}
	return &u, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != pgerrcode.UniqueViolation {
		return false
	}
	if constraint == "" {
		return true
	}
	return pgErr.ConstraintName == constraint
}
