package bcrypt

import (
	"errors"
	"fmt"

	gobcrypt "golang.org/x/crypto/bcrypt"
)

// ErrMismatch пароль не совпал с хэшем
var ErrMismatch = errors.New("password mismatch")

// Hasher хэширует и сверяет пароли
type Hasher interface {
	// Hash возвращает хэш пароля
	Hash(password string) (string, error)
	// Compare сверяет пароль с хэшем
	Compare(hash, password string) error
}

type bcryptHasher struct {
	cost int
}

// New создаёт Hasher с указанной стоимостью bcrypt
func New(cost int) Hasher {
	if cost < gobcrypt.MinCost {
		cost = gobcrypt.DefaultCost
	}
	return &bcryptHasher{cost: cost}
}

// Hash возвращает bcrypt-хэш пароля
func (h *bcryptHasher) Hash(password string) (string, error) {
	out, err := gobcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt generate: %w", err)
	}
	return string(out), nil
}

// Compare возвращает nil при совпадении, ErrMismatch при несовпадении
func (h *bcryptHasher) Compare(hash, password string) error {
	err := gobcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return nil
	}
	if errors.Is(err, gobcrypt.ErrMismatchedHashAndPassword) {
		return ErrMismatch
	}
	return fmt.Errorf("bcrypt compare: %w", err)
}
