package http

import (
	"fmt"

	"github.com/example/ai-restaurant-assistant-backend/internal/audit"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
)

// intStr — короткая обёртка для TargetID (int → "string"), чтобы не таскать
// strconv по всем методам.
func intStr(n int) string {
	return fmt.Sprintf("%d", n)
}

// categoryPatchChanges собирает дифф из patch-DTO. Хранятся только заполненные поля.
// Реальные старые значения у нас сюда не приезжают (usecase их не возвращает) —
// в логе для PATCH-операций показываем только "to".
func categoryPatchChanges(p apimodels.PatchCategoryRequest) []audit.Change {
	out := []audit.Change{}
	if p.Name != nil {
		out = append(out, audit.Change{Field: "name", To: *p.Name})
	}
	if p.SortOrder != nil {
		out = append(out, audit.Change{Field: "sort_order", To: fmt.Sprintf("%d", *p.SortOrder)})
	}
	if p.IsAvailable != nil {
		out = append(out, audit.Change{Field: "is_available", To: boolStr(*p.IsAvailable)})
	}
	if p.Role != nil {
		out = append(out, audit.Change{Field: "role", To: string(*p.Role)})
	}
	return out
}

// tagPatchChanges — дифф для PATCH /admin/tags/{id}.
func tagPatchChanges(p apimodels.PatchTagRequest) []audit.Change {
	out := []audit.Change{}
	if p.Name != nil {
		out = append(out, audit.Change{Field: "name", To: *p.Name})
	}
	if p.Slug != nil {
		out = append(out, audit.Change{Field: "slug", To: *p.Slug})
	}
	if p.Color != nil {
		out = append(out, audit.Change{Field: "color", To: *p.Color})
	}
	return out
}

// dishPatchChanges — дифф для PATCH /admin/menu/{id}.
// Для массивов и сложных полей пишем только сам факт изменения, без значений
// (значения не помещаются в краткую строку — видны через GET /admin/menu/{id}).
func dishPatchChanges(p apimodels.PatchDishRequest) []audit.Change {
	out := []audit.Change{}
	if p.Name != nil {
		out = append(out, audit.Change{Field: "name", To: *p.Name})
	}
	if p.PriceMinor != nil {
		out = append(out, audit.Change{Field: "price_minor", To: fmt.Sprintf("%d", *p.PriceMinor)})
	}
	if p.IsAvailable != nil {
		out = append(out, audit.Change{Field: "is_available", To: boolStr(*p.IsAvailable)})
	}
	if p.CategoryId != nil {
		out = append(out, audit.Change{Field: "category_id", To: fmt.Sprintf("%d", *p.CategoryId)})
	}
	if p.Description != nil {
		out = append(out, audit.Change{Field: "description", To: "(изменено)"})
	}
	if p.Composition != nil {
		out = append(out, audit.Change{Field: "composition", To: "(изменено)"})
	}
	if p.Cuisine != nil {
		out = append(out, audit.Change{Field: "cuisine", To: string(*p.Cuisine)})
	}
	if p.Allergens != nil {
		out = append(out, audit.Change{Field: "allergens", To: "(изменено)"})
	}
	if p.Dietary != nil {
		out = append(out, audit.Change{Field: "dietary", To: "(изменено)"})
	}
	if p.TagIds != nil {
		out = append(out, audit.Change{Field: "tag_ids", To: "(изменено)"})
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
