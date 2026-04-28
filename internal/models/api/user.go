package api

import (
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ProfileFromUsecase маппит usecase.User в api.Profile
func ProfileFromUsecase(u *usecasemodels.User) Profile {
	p := Profile{
		Email:     openapi_types.Email(u.Email),
		Allergens: coalesceSlice(u.Allergens),
		Dietary:   coalesceSlice(u.Dietary),
	}
	if u.FirstName != "" {
		v := u.FirstName
		p.FirstName = &v
	}
	if u.LastName != "" {
		v := u.LastName
		p.LastName = &v
	}
	if u.Phone != "" {
		v := u.Phone
		p.Phone = &v
	}
	return p
}

// PatchProfileRequestToUsecase маппит api.PatchProfileRequest в usecase.ProfilePatch
func PatchProfileRequestToUsecase(req PatchProfileRequest) usecasemodels.ProfilePatch {
	return usecasemodels.ProfilePatch{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Allergens: req.Allergens,
		Dietary:   req.Dietary,
	}
}

func coalesceSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
