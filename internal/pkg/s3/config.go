package s3

// Config параметры S3-совместимого хранилища
type Config struct {
	// Endpoint host:port S3-совместимого API
	Endpoint string `yaml:"endpoint"`
	// AccessKey ключ доступа
	AccessKey string `yaml:"access_key"`
	// SecretKey секретный ключ
	SecretKey string `yaml:"secret_key"`
	// Bucket имя бакета
	Bucket string `yaml:"bucket"`
	// Region регион (для S3 — обязателен; для MinIO — любой)
	Region string `yaml:"region"`
	// UseSSL использовать https
	UseSSL bool `yaml:"use_ssl"`
	// PublicBaseURL базовый URL для публичных ссылок (например, http://localhost:9000/restaurant)
	PublicBaseURL string `yaml:"public_base_url"`
}
