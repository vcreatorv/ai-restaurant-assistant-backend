package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage S3-совместимое хранилище объектов
type Storage interface {
	// Upload заливает объект и возвращает публичный URL
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error)
	// Delete удаляет объект по ключу
	Delete(ctx context.Context, key string) error
}

type minioStorage struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

// New создаёт Storage поверх minio-go
func New(cfg Config) (Storage, error) {
	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	if base == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://%s/%s", scheme, cfg.Endpoint, cfg.Bucket)
	}
	return &minioStorage{client: cli, bucket: cfg.Bucket, publicBaseURL: base}, nil
}

// Upload заливает объект и возвращает публичный URL
func (s *minioStorage) Upload(
	ctx context.Context,
	key, contentType string,
	body io.Reader,
	size int64,
) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	return s.publicBaseURL + "/" + key, nil
}

// Delete удаляет объект по ключу
func (s *minioStorage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}
