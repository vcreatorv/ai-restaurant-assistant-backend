package api

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// PromptVersionFromUsecase конвертирует prompts.Version в API-DTO.
func PromptVersionFromUsecase(v prompts.Version) PromptVersion {
	return PromptVersion{
		Version:     v.Version,
		Content:     v.Content,
		PublishedAt: v.PublishedAt,
		PublishedBy: PromptAuthor{
			Id:          openapi_types.UUID(v.PublishedBy.ID),
			DisplayName: v.PublishedBy.DisplayName,
			Email:       openapi_types.Email(v.PublishedBy.Email),
		},
	}
}

// PromptDraftFromUsecase конвертирует prompts.Draft в API-DTO.
func PromptDraftFromUsecase(d *prompts.Draft) *PromptDraft {
	if d == nil {
		return nil
	}
	return &PromptDraft{
		Name:      PromptName(d.Name),
		Content:   d.Content,
		UpdatedAt: d.UpdatedAt,
	}
}

// PromptFromUsecase конвертирует prompts.Prompt в API-DTO.
func PromptFromUsecase(p prompts.Prompt) Prompt {
	return Prompt{
		Name:    PromptName(p.Name),
		Current: PromptVersionFromUsecase(p.Current),
		Draft:   PromptDraftFromUsecase(p.Draft),
	}
}

// PromptListFromUsecase конвертирует список промптов в API-DTO.
func PromptListFromUsecase(items []prompts.Prompt) PromptList {
	out := PromptList{Items: make([]Prompt, len(items))}
	for i, p := range items {
		out.Items[i] = PromptFromUsecase(p)
	}
	return out
}

// PromptDetailsFromUsecase конвертирует prompts.Details в API-DTO.
func PromptDetailsFromUsecase(d prompts.Details) PromptDetails {
	history := make([]PromptVersion, len(d.History))
	for i, v := range d.History {
		history[i] = PromptVersionFromUsecase(v)
	}
	return PromptDetails{
		Name:    PromptName(d.Name),
		Current: PromptVersionFromUsecase(d.Current),
		Draft:   PromptDraftFromUsecase(d.Draft),
		History: history,
	}
}
