package datasources

// PostgresConfig параметры подключения к PostgreSQL
type PostgresConfig struct {
	// DSN строка подключения
	DSN string `yaml:"dsn"`
}

// RedisConfig параметры подключения к Redis
type RedisConfig struct {
	// Addr адрес сервера
	Addr string `yaml:"addr"`
	// Password пароль
	Password string `yaml:"password"`
	// DB номер БД
	DB int `yaml:"db"`
}
