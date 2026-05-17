package api

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// AdminActionFromUsecase конвертирует audit.Action в API-DTO.
func AdminActionFromUsecase(a audit.Action) AdminAction {
	out := AdminAction{
		Id:          a.ID,
		Target:      AdminActionTarget(a.Target),
		TargetId:    a.TargetID,
		TargetLabel: a.TargetLabel,
		Verb:        AdminActionVerb(a.Verb),
		CreatedAt:   a.CreatedAt,
		Admin: AdminActionAuthor{
			DisplayName: a.Admin.DisplayName,
			HasNamesake: a.Admin.HasNamesake,
		},
		Changes: make([]AdminActionChange, len(a.Changes)),
	}
	if a.Admin.ID != nil {
		uid := openapi_types.UUID(*a.Admin.ID)
		out.Admin.Id = &uid
	}
	if a.Admin.Email != nil {
		em := openapi_types.Email(*a.Admin.Email)
		out.Admin.Email = &em
	}
	for i, c := range a.Changes {
		ch := AdminActionChange{Field: c.Field}
		if c.From != "" {
			from := c.From
			ch.From = &from
		}
		if c.To != "" {
			to := c.To
			ch.To = &to
		}
		out.Changes[i] = ch
	}
	return out
}

// AdminActionListFromUsecase упаковывает в DTO с пагинацией.
func AdminActionListFromUsecase(items []audit.Action, total, limit, offset int) AdminActionList {
	out := AdminActionList{
		Items:  make([]AdminAction, len(items)),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
	for i, a := range items {
		out.Items[i] = AdminActionFromUsecase(a)
	}
	return out
}
