package repository

// Session сессия в storage-форме
type Session struct {
	// UserID идентификатор привязанного пользователя
	UserID *string `json:"user_id"`
	// CSRF токен для защиты от CSRF
	CSRF string `json:"csrf"`
}
