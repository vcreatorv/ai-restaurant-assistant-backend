// Package postgres содержит PostgreSQL-реализацию chat.Repository.
package postgres

import "github.com/jackc/pgx/v5/pgxpool"

// Repository PostgreSQL-репозиторий чатов
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
