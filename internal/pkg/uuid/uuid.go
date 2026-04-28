package uuid

import googleuuid "github.com/google/uuid"

// Generator генератор UUID
type Generator interface {
	// New возвращает новый UUID
	New() googleuuid.UUID
}

type stdGenerator struct{}

// New создаёт генератор UUIDv4
func New() Generator { return &stdGenerator{} }

// New генерирует UUIDv4
func (stdGenerator) New() googleuuid.UUID { return googleuuid.New() }
