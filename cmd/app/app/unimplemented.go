package app

import (
	"context"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
)

// Unimplemented заглушки для нереализованных endpoint'ов
type Unimplemented struct{}

func (Unimplemented) AdminGetAnalyticsOverview(_ context.Context, _ v1.AdminGetAnalyticsOverviewRequestObject) (v1.AdminGetAnalyticsOverviewResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}

func (Unimplemented) AdminListAnalyticsQueries(_ context.Context, _ v1.AdminListAnalyticsQueriesRequestObject) (v1.AdminListAnalyticsQueriesResponseObject, error) {
	return nil, apperrors.ErrNotImplemented
}
