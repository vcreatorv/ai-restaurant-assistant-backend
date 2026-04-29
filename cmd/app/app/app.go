package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	v1 "github.com/example/ai-restaurant-assistant-backend/cmd/app/app/v1"
	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	authhttp "github.com/example/ai-restaurant-assistant-backend/internal/auth/delivery/v1/http"
	authusecase "github.com/example/ai-restaurant-assistant-backend/internal/auth/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	chathttp "github.com/example/ai-restaurant-assistant-backend/internal/chat/delivery/v1/http"
	chatpostgres "github.com/example/ai-restaurant-assistant-backend/internal/chat/repository/postgres"
	chatusecase "github.com/example/ai-restaurant-assistant-backend/internal/chat/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	menuhttp "github.com/example/ai-restaurant-assistant-backend/internal/menu/delivery/v1/http"
	menupostgres "github.com/example/ai-restaurant-assistant-backend/internal/menu/repository/postgres"
	menuusecase "github.com/example/ai-restaurant-assistant-backend/internal/menu/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/apperrors"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/bcrypt"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/csrf"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/datasources"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/middleware"
	pkgredis "github.com/example/ai-restaurant-assistant-backend/internal/pkg/redis"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/s3"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/uuid"
	sessionredis "github.com/example/ai-restaurant-assistant-backend/internal/session/repository/redis"
	sessionusecase "github.com/example/ai-restaurant-assistant-backend/internal/session/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
	userhttp "github.com/example/ai-restaurant-assistant-backend/internal/user/delivery/v1/http"
	userpostgres "github.com/example/ai-restaurant-assistant-backend/internal/user/repository/postgres"
	userusecase "github.com/example/ai-restaurant-assistant-backend/internal/user/usecase"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/jackc/pgx/v5/pgxpool"

	oapimw "github.com/oapi-codegen/nethttp-middleware"

	goredis "github.com/redis/go-redis/v9"
)

// Middleware HTTP middleware в форме decorator
type Middleware func(http.Handler) http.Handler

// App собранное приложение
type App struct {
	cfg         *Config
	server      *http.Server
	redisClient *goredis.Client
	pgPool      *pgxpool.Pool
}

// New собирает зависимости и подготавливает HTTP-сервер
func New(cfg *Config) (*App, error) {
	log := logger.New(cfg.Log.Level)

	pgPool, err := datasources.NewPostgresPool(context.Background(), cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	redisClient, err := datasources.NewRedisClient(context.Background(), cfg.Redis)
	if err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}

	handler, middlewares, err := buildAPI(cfg, log, pgPool, redisClient)
	if err != nil {
		_ = redisClient.Close()
		pgPool.Close()
		return nil, err
	}

	router := buildRouter(handler, middlewares)

	return &App{
		cfg:         cfg,
		redisClient: redisClient,
		pgPool:      pgPool,
		server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}, nil
}

// Run запускает сервер до отмены context или фатальной ошибки
func (a *App) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		_ = a.redisClient.Close()
		a.pgPool.Close()
		return nil
	case err := <-serverErr:
		return err
	}
}

// buildAPI инициализирует фичи и собирает Handler + middleware-цепочку
func buildAPI(
	cfg *Config,
	log *slog.Logger,
	pgPool *pgxpool.Pool,
	redisClient *goredis.Client,
) (Handler, []Middleware, error) {
	redisManager := pkgredis.NewRedis(redisClient)

	uuidGen := uuid.New()
	csrfGen := csrf.New()
	bcryptHasher := bcrypt.New(cfg.Auth.Usecase.BcryptCost)

	s3Storage, err := s3.New(cfg.S3)
	if err != nil {
		return Handler{}, nil, fmt.Errorf("s3: %w", err)
	}

	sessionRepository := sessionredis.New(redisManager, cfg.Session.Repository.TTL)
	userRepository := userpostgres.New(pgPool)
	menuRepository := menupostgres.New(pgPool)
	chatRepository := chatpostgres.New(pgPool)

	sessionUsecase := sessionusecase.New(sessionRepository, uuidGen, csrfGen)
	userUsecase := userusecase.New(userRepository)
	authUsecase := authusecase.New(userRepository, sessionUsecase, bcryptHasher, uuidGen)
	menuUsecase := menuusecase.New(menuRepository, s3Storage)
	chatUsecase := chatusecase.New(chatRepository, uuidGen, cfg.Chat.Usecase)

	authHandler := authhttp.New(authUsecase, userUsecase)
	userHandler := userhttp.New(userUsecase)
	menuHandler := menuhttp.New(cfg.Menu.Delivery, menuUsecase, userUsecase)
	chatHandler := chathttp.New(cfg.Chat.Delivery, chatUsecase)

	handler := Handler{
		AuthHandler: authHandler,
		UserHandler: userHandler,
		MenuHandler: menuHandler,
		ChatHandler: chatHandler,
	}

	swagger, err := v1.GetSwagger()
	if err != nil {
		return Handler{}, nil, fmt.Errorf("openapi spec: %w", err)
	}
	swagger.Servers = openapi3.Servers{{URL: "/api/v1"}}

	middlewares := []Middleware{
		middleware.Recovery,
		middleware.RequestID,
		middleware.Logger(log),
		middleware.Session(sessionUsecase, cfg.Session.Repository.TTL, cfg.HTTP.CookieSecure),
		middleware.CSRF,
		oapimw.OapiRequestValidatorWithOptions(swagger, &oapimw.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
			SilenceServersWarning: true,
		}),
	}

	return handler, middlewares, nil
}

// buildRouter навешивает middleware на strict-server в порядке списка (первый — внешний)
func buildRouter(handler Handler, middlewares []Middleware) http.Handler {
	strict := v1.NewStrictHandlerWithOptions(handler, nil, v1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler,
	})

	mux := http.NewServeMux()
	api := v1.HandlerWithOptions(strict, v1.StdHTTPServerOptions{
		BaseURL:          "/api/v1",
		BaseRouter:       mux,
		ErrorHandlerFunc: requestErrorHandler,
	})

	h := api
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// requestErrorHandler обрабатывает ошибки парсинга и валидации запроса
func requestErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
}

// responseErrorHandler маппит ошибки бизнес-логики в HTTP-ответы
func responseErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	switch {
	case errors.Is(err, apperrors.ErrNotImplemented):
		writeError(w, http.StatusNotImplemented, "not_implemented", "This endpoint is not yet implemented")
	case errors.Is(err, apperrors.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
	case errors.Is(err, apperrors.ErrForbidden):
		writeError(w, http.StatusForbidden, "access_denied", "Access denied")
	case errors.Is(err, apperrors.ErrBadRequest):
		writeError(w, http.StatusBadRequest, "validation_failed", "Invalid request")
	case errors.Is(err, apperrors.ErrInternalNoSession):
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")

	case errors.Is(err, user.ErrNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "User not found")
	case errors.Is(err, user.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_already_taken", "Email is already registered")
	case errors.Is(err, user.ErrInsufficientRole):
		writeError(w, http.StatusForbidden, "access_denied", "Operation not allowed for this role")

	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
	case errors.Is(err, auth.ErrAlreadyRegistered):
		writeError(w, http.StatusConflict, "already_registered", "User is already registered")

	case errors.Is(err, menu.ErrCategoryNotFound):
		writeError(w, http.StatusNotFound, "category_not_found", "Category not found")
	case errors.Is(err, menu.ErrCategoryNameTaken):
		writeError(w, http.StatusConflict, "category_name_taken", "Category with this name already exists")
	case errors.Is(err, menu.ErrCategoryHasDishes):
		writeError(w, http.StatusConflict, "category_has_dishes", "Category contains dishes — cannot delete")
	case errors.Is(err, menu.ErrTagNotFound):
		writeError(w, http.StatusNotFound, "tag_not_found", "Tag not found")
	case errors.Is(err, menu.ErrTagNameTaken):
		writeError(w, http.StatusConflict, "tag_name_taken", "Tag with this name or slug already exists")
	case errors.Is(err, menu.ErrDishNotFound):
		writeError(w, http.StatusNotFound, "dish_not_found", "Dish not found")
	case errors.Is(err, menu.ErrDishNameTaken):
		writeError(w, http.StatusConflict, "dish_name_taken", "Dish with this name already exists")
	case errors.Is(err, menu.ErrInvalidCuisine):
		writeError(w, http.StatusBadRequest, "invalid_cuisine", "Invalid cuisine value")
	case errors.Is(err, menu.ErrImageTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", "Image exceeds maximum allowed size")
	case errors.Is(err, menu.ErrImageUnsupportedType):
		writeError(w, http.StatusUnsupportedMediaType, "image_unsupported_type", "Unsupported image content type")

	case errors.Is(err, chat.ErrChatNotFound):
		writeError(w, http.StatusNotFound, "chat_not_found", "Chat not found")
	case errors.Is(err, chat.ErrChatForbidden):
		writeError(w, http.StatusForbidden, "access_denied", "Chat does not belong to this user")
	case errors.Is(err, chat.ErrEmptyMessage):
		writeError(w, http.StatusBadRequest, "validation_failed", "Message content is empty")

	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
