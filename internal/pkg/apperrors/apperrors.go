package apperrors

import "errors"

var (
	// ErrUnauthenticated требуется авторизация
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden недостаточно прав
	ErrForbidden = errors.New("forbidden")
	// ErrBadRequest некорректный запрос
	ErrBadRequest = errors.New("bad_request")
	// ErrNotImplemented endpoint не реализован
	ErrNotImplemented = errors.New("not_implemented")
	// ErrInternalNoSession middleware не положил сессию в context
	ErrInternalNoSession = errors.New("session middleware did not set session in context")
)
