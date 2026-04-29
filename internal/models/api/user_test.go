package api

import (
	"testing"

	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestProfileFromUsecase_FullCustomer(t *testing.T) {
	u := &usecasemodels.User{
		Email:     "ivan@example.com",
		FirstName: "Иван",
		LastName:  "Петров",
		Phone:     "+79001234567",
		Allergens: []string{"dairy"},
		Dietary:   []string{"halal"},
	}
	p := ProfileFromUsecase(u)

	require.Equal(t, "ivan@example.com", string(p.Email))
	require.Equal(t, ptr("Иван"), p.FirstName)
	require.Equal(t, ptr("Петров"), p.LastName)
	require.Equal(t, ptr("+79001234567"), p.Phone)
	require.Equal(t, []string{"dairy"}, p.Allergens)
}

func TestProfileFromUsecase_EmptyOptionals_PointersAreNil(t *testing.T) {
	u := &usecasemodels.User{Email: "ivan@example.com"}
	p := ProfileFromUsecase(u)

	require.Nil(t, p.FirstName, "пустая строка → nil")
	require.Nil(t, p.LastName)
	require.Nil(t, p.Phone)
	require.Equal(t, []string{}, p.Allergens, "nil слайс → пустой")
	require.Equal(t, []string{}, p.Dietary)
}

func TestPatchProfileRequestToUsecase_PassThrough(t *testing.T) {
	req := PatchProfileRequest{
		FirstName: ptr("Иван"),
		Phone:     ptr("+79001234567"),
		Allergens: ptr([]string{"shellfish"}),
	}
	patch := PatchProfileRequestToUsecase(req)

	require.Equal(t, ptr("Иван"), patch.FirstName)
	require.Nil(t, patch.LastName, "не задано → nil pointer")
	require.Equal(t, ptr("+79001234567"), patch.Phone)
	require.Equal(t, ptr([]string{"shellfish"}), patch.Allergens)
}
