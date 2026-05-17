package app

// Unimplemented — заглушка для embedding в Handler, на случай если в OpenAPI появятся
// новые endpoint'ы быстрее, чем будут реализованы. Сейчас все методы StrictServerInterface
// покрываются конкретными handler'ами; поле оставлено как точка расширения.
type Unimplemented struct{}
