// Package postgres реализует cart.Repository поверх PostgreSQL (pgx/v5).
package postgres

import "github.com/jackc/pgx/v5/pgxpool"

// Repository PostgreSQL-репозиторий корзины
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
