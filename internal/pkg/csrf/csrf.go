package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Generator генератор CSRF-токенов
type Generator interface {
	// New возвращает новый CSRF-токен
	New() string
}

type stdGenerator struct {
	bytes int
}

// New создаёт Generator с длиной токена 32 байта
func New() Generator { return &stdGenerator{bytes: 32} }

// New генерирует CSRF-токен в base64 URL-safe
func (g *stdGenerator) New() string {
	b := make([]byte, g.bytes)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
