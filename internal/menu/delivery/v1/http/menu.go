package http

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	apimodels "github.com/example/ai-restaurant-assistant-backend/internal/models/api"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
)

// ListCategories реализует GET /categories
func (h MenuHandler) ListCategories(
	ctx context.Context,
	_ v1.ListCategoriesRequestObject,
) (v1.ListCategoriesResponseObject, error) {
	cs, err := h.usecase.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return v1.ListCategories200JSONResponse(apimodels.CategoryListFromUsecase(cs)), nil
}

// ListDishes реализует GET /menu
func (h MenuHandler) ListDishes(
	ctx context.Context,
	request v1.ListDishesRequestObject,
) (v1.ListDishesResponseObject, error) {
	h.fillListDishesDefaults(&request.Params)
	f := apimodels.ListDishesParamsToUsecase(request.Params)
	dishes, total, err := h.usecase.ListDishes(ctx, f)
	if err != nil {
		return nil, err
	}
	return v1.ListDishes200JSONResponse(apimodels.DishListFromUsecase(dishes, total, f.Limit, f.Offset)), nil
}

// GetDish реализует GET /menu/{id}
func (h MenuHandler) GetDish(
	ctx context.Context,
	request v1.GetDishRequestObject,
) (v1.GetDishResponseObject, error) {
	d, err := h.usecase.GetDish(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return v1.GetDish200JSONResponse(apimodels.DishFromUsecase(d)), nil
}

// AdminCreateCategory реализует POST /admin/categories
func (h MenuHandler) AdminCreateCategory(
	ctx context.Context,
	request v1.AdminCreateCategoryRequestObject,
) (v1.AdminCreateCategoryResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	c, err := h.usecase.CreateCategory(ctx, apimodels.CreateCategoryRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminCreateCategory201JSONResponse(apimodels.CategoryFromUsecase(*c)), nil
}

// AdminUpdateCategory реализует PATCH /admin/categories/{id}
func (h MenuHandler) AdminUpdateCategory(
	ctx context.Context,
	request v1.AdminUpdateCategoryRequestObject,
) (v1.AdminUpdateCategoryResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	c, err := h.usecase.UpdateCategory(ctx, request.Id, apimodels.PatchCategoryRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminUpdateCategory200JSONResponse(apimodels.CategoryFromUsecase(*c)), nil
}

// AdminDeleteCategory реализует DELETE /admin/categories/{id}
func (h MenuHandler) AdminDeleteCategory(
	ctx context.Context,
	request v1.AdminDeleteCategoryRequestObject,
) (v1.AdminDeleteCategoryResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.usecase.DeleteCategory(ctx, request.Id); err != nil {
		return nil, err
	}
	return v1.AdminDeleteCategory204Response{}, nil
}

// AdminListTags реализует GET /admin/tags
func (h MenuHandler) AdminListTags(
	ctx context.Context,
	_ v1.AdminListTagsRequestObject,
) (v1.AdminListTagsResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	ts, err := h.usecase.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	return v1.AdminListTags200JSONResponse(apimodels.TagListFromUsecase(ts)), nil
}

// AdminCreateTag реализует POST /admin/tags
func (h MenuHandler) AdminCreateTag(
	ctx context.Context,
	request v1.AdminCreateTagRequestObject,
) (v1.AdminCreateTagResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	t, err := h.usecase.CreateTag(ctx, apimodels.CreateTagRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminCreateTag201JSONResponse(apimodels.TagFromUsecase(*t)), nil
}

// AdminUpdateTag реализует PATCH /admin/tags/{id}
func (h MenuHandler) AdminUpdateTag(
	ctx context.Context,
	request v1.AdminUpdateTagRequestObject,
) (v1.AdminUpdateTagResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	t, err := h.usecase.UpdateTag(ctx, request.Id, apimodels.PatchTagRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminUpdateTag200JSONResponse(apimodels.TagFromUsecase(*t)), nil
}

// AdminDeleteTag реализует DELETE /admin/tags/{id}
func (h MenuHandler) AdminDeleteTag(
	ctx context.Context,
	request v1.AdminDeleteTagRequestObject,
) (v1.AdminDeleteTagResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.usecase.DeleteTag(ctx, request.Id); err != nil {
		return nil, err
	}
	return v1.AdminDeleteTag204Response{}, nil
}

// AdminCreateDish реализует POST /admin/menu
func (h MenuHandler) AdminCreateDish(
	ctx context.Context,
	request v1.AdminCreateDishRequestObject,
) (v1.AdminCreateDishResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	d, err := h.usecase.CreateDish(ctx, apimodels.CreateDishRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminCreateDish201JSONResponse(apimodels.DishFromUsecase(d)), nil
}

// AdminUpdateDish реализует PATCH /admin/menu/{id}
func (h MenuHandler) AdminUpdateDish(
	ctx context.Context,
	request v1.AdminUpdateDishRequestObject,
) (v1.AdminUpdateDishResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	d, err := h.usecase.UpdateDish(ctx, request.Id, apimodels.PatchDishRequestToUsecase(*request.Body))
	if err != nil {
		return nil, err
	}
	return v1.AdminUpdateDish200JSONResponse(apimodels.DishFromUsecase(d)), nil
}

// AdminDeleteDish реализует DELETE /admin/menu/{id}
func (h MenuHandler) AdminDeleteDish(
	ctx context.Context,
	request v1.AdminDeleteDishRequestObject,
) (v1.AdminDeleteDishResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.usecase.DeleteDish(ctx, request.Id); err != nil {
		return nil, err
	}
	return v1.AdminDeleteDish204Response{}, nil
}

// AdminUploadDishImage реализует POST /admin/menu/{id}/image
func (h MenuHandler) AdminUploadDishImage(
	ctx context.Context,
	request v1.AdminUploadDishImageRequestObject,
) (v1.AdminUploadDishImageResponseObject, error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, apperrors.ErrBadRequest
	}
	src, cleanup, err := readImagePart(request.Body, h.cfg.MaxImageSizeBytes)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	d, err := h.usecase.UploadDishImage(ctx, request.Id, src)
	if err != nil {
		return nil, err
	}
	return v1.AdminUploadDishImage200JSONResponse(apimodels.DishFromUsecase(d)), nil
}
