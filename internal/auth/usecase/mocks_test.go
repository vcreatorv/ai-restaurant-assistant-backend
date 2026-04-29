package usecase

import (
	"context"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/google/uuid"
)

type mockUserRepo struct {
	findByIDFn    func(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error)
	findByEmailFn func(ctx context.Context, email string) (*repositorymodels.User, error)
	createFn      func(ctx context.Context, u *repositorymodels.User) error
	updateFn      func(ctx context.Context, u *repositorymodels.User) error
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error) {
	return m.findByIDFn(ctx, id)
}
func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*repositorymodels.User, error) {
	return m.findByEmailFn(ctx, email)
}
func (m *mockUserRepo) Create(ctx context.Context, u *repositorymodels.User) error {
	return m.createFn(ctx, u)
}
func (m *mockUserRepo) Update(ctx context.Context, u *repositorymodels.User) error {
	return m.updateFn(ctx, u)
}

type mockSessionUC struct {
	ensureFn     func(ctx context.Context, id *uuid.UUID) (*usecasemodels.Session, error)
	attachUserFn func(ctx context.Context, sessionID, userID uuid.UUID) (*usecasemodels.Session, error)
	destroyFn    func(ctx context.Context, sessionID uuid.UUID) error
}

func (m *mockSessionUC) Ensure(ctx context.Context, id *uuid.UUID) (*usecasemodels.Session, error) {
	return m.ensureFn(ctx, id)
}
func (m *mockSessionUC) AttachUser(ctx context.Context, sessionID, userID uuid.UUID) (*usecasemodels.Session, error) {
	return m.attachUserFn(ctx, sessionID, userID)
}
func (m *mockSessionUC) Destroy(ctx context.Context, sessionID uuid.UUID) error {
	return m.destroyFn(ctx, sessionID)
}

type mockHasher struct {
	hashFn    func(password string) (string, error)
	compareFn func(hash, password string) error
}

func (m *mockHasher) Hash(password string) (string, error) { return m.hashFn(password) }
func (m *mockHasher) Compare(hash, password string) error  { return m.compareFn(hash, password) }

type mockUUID struct{ next uuid.UUID }

func (m *mockUUID) New() uuid.UUID { return m.next }
