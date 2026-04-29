package usecase

import (
	"testing"

	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestUserFromRepository_FullCustomer(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	r := &repositorymodels.User{
		ID:           id,
		Email:        ptr("ivan@example.com"),
		PasswordHash: ptr("$2a$12$abc"),
		Role:         "customer",
		FirstName:    ptr("Иван"),
		LastName:     ptr("Петров"),
		Phone:        ptr("+79001234567"),
		Allergens:    []string{"dairy", "gluten"},
		Dietary:      []string{"halal"},
	}
	u := UserFromRepository(r)

	require.Equal(t, id, u.ID)
	require.Equal(t, "ivan@example.com", u.Email)
	require.Equal(t, RoleCustomer, u.Role)
	require.Equal(t, "Иван", u.FirstName)
	require.Equal(t, []string{"dairy", "gluten"}, u.Allergens)
	require.True(t, u.IsCustomer())
}

func TestUserFromRepository_GuestNullables(t *testing.T) {
	r := &repositorymodels.User{
		ID:        uuid.New(),
		Role:      "guest",
		Allergens: nil,
		Dietary:   nil,
	}
	u := UserFromRepository(r)

	require.Empty(t, u.Email)
	require.Empty(t, u.PasswordHash)
	require.Empty(t, u.FirstName)
	require.Equal(t, []string{}, u.Allergens, "nil слайсы должны нормализоваться в пустой")
	require.Equal(t, []string{}, u.Dietary)
	require.True(t, u.IsGuest())
}

func TestUserFromRepository_Nil(t *testing.T) {
	require.Nil(t, UserFromRepository(nil))
}

func TestUserToRepository_RoundTrip(t *testing.T) {
	id := uuid.New()
	src := &User{
		ID:        id,
		Email:     "ivan@example.com",
		Role:      RoleCustomer,
		FirstName: "Иван",
		Phone:     "+79001234567",
		Allergens: []string{"nuts"},
		Dietary:   []string{},
	}
	r := UserToRepository(src)

	require.Equal(t, id, r.ID)
	require.Equal(t, ptr("ivan@example.com"), r.Email)
	require.Equal(t, ptr("Иван"), r.FirstName)
	require.Nil(t, r.LastName, "пустая строка должна стать nil")
	require.Equal(t, ptr("+79001234567"), r.Phone)
	require.Equal(t, []string{"nuts"}, r.Allergens)
}

func TestUser_IsRoleHelpers(t *testing.T) {
	cases := map[Role]struct{ guest, customer, admin bool }{
		RoleGuest:    {true, false, false},
		RoleCustomer: {false, true, false},
		RoleAdmin:    {false, false, true},
	}
	for role, want := range cases {
		t.Run(string(role), func(t *testing.T) {
			u := &User{Role: role}
			require.Equal(t, want.guest, u.IsGuest())
			require.Equal(t, want.customer, u.IsCustomer())
			require.Equal(t, want.admin, u.IsAdmin())
		})
	}
}
