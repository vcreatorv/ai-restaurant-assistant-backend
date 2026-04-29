package usecase

import (
	"context"
	"errors"
	"testing"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func userID() uuid.UUID { return uuid.MustParse("11111111-1111-1111-1111-111111111111") }

type mockRepo struct {
	findByIDFn    func(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error)
	findByEmailFn func(ctx context.Context, email string) (*repositorymodels.User, error)
	createFn      func(ctx context.Context, u *repositorymodels.User) error
	updateFn      func(ctx context.Context, u *repositorymodels.User) error
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*repositorymodels.User, error) {
	return m.findByIDFn(ctx, id)
}
func (m *mockRepo) FindByEmail(ctx context.Context, email string) (*repositorymodels.User, error) {
	return m.findByEmailFn(ctx, email)
}
func (m *mockRepo) Create(ctx context.Context, u *repositorymodels.User) error {
	return m.createFn(ctx, u)
}
func (m *mockRepo) Update(ctx context.Context, u *repositorymodels.User) error {
	return m.updateFn(ctx, u)
}

func TestGetByID_HappyPath(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*repositorymodels.User, error) {
			require.Equal(t, userID(), id)
			return &repositorymodels.User{ID: id, Email: ptr("ivan@example.com"), Role: "customer"}, nil
		},
	}
	uc := New(repo)

	u, err := uc.GetByID(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, "ivan@example.com", u.Email)
	require.True(t, u.IsCustomer())
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return nil, user.ErrNotFound
		},
	}
	uc := New(repo)
	_, err := uc.GetByID(context.Background(), userID())
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestGetProfile_GuestForbidden(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), Role: "guest"}, nil
		},
	}
	uc := New(repo)
	_, err := uc.GetProfile(context.Background(), userID())
	require.ErrorIs(t, err, user.ErrInsufficientRole, "профиль доступен только customer/admin")
}

func TestGetProfile_CustomerOK(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{
				ID: userID(), Email: ptr("ivan@example.com"),
				Role: "customer", FirstName: ptr("Иван"),
			}, nil
		},
	}
	uc := New(repo)

	u, err := uc.GetProfile(context.Background(), userID())
	require.NoError(t, err)
	require.Equal(t, "Иван", u.FirstName)
}

func TestUpdateProfile_PartialPatch(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{
				ID:        userID(),
				Email:     ptr("ivan@example.com"),
				Role:      "customer",
				FirstName: ptr("Старое"),
				Phone:     ptr("+79000000000"),
				Allergens: []string{"dairy"},
			}, nil
		},
		updateFn: func(_ context.Context, u *repositorymodels.User) error {
			require.Equal(t, ptr("Иван"), u.FirstName, "обновлено")
			require.Equal(t, ptr("+79000000000"), u.Phone, "не трогалось")
			require.Equal(t, []string{"shellfish"}, u.Allergens, "перезатёрто")
			return nil
		},
	}
	uc := New(repo)

	out, err := uc.UpdateProfile(context.Background(), userID(), usecasemodels.ProfilePatch{
		FirstName: ptr("Иван"),
		Allergens: ptr([]string{"shellfish"}),
	})
	require.NoError(t, err)
	require.Equal(t, "Иван", out.FirstName)
}

func TestUpdateProfile_GuestForbidden(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), Role: "guest"}, nil
		},
	}
	uc := New(repo)
	_, err := uc.UpdateProfile(context.Background(), userID(), usecasemodels.ProfilePatch{FirstName: ptr("X")})
	require.ErrorIs(t, err, user.ErrInsufficientRole)
}

func TestUpdateProfile_RepoUpdateError_Bubbles(t *testing.T) {
	repo := &mockRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), Email: ptr("x"), Role: "customer"}, nil
		},
		updateFn: func(context.Context, *repositorymodels.User) error { return errors.New("db down") },
	}
	uc := New(repo)
	_, err := uc.UpdateProfile(context.Background(), userID(), usecasemodels.ProfilePatch{FirstName: ptr("X")})
	require.Error(t, err)
}
