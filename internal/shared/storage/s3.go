package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/yourorg/enterprise-api/internal/config"
)

// S3CompatibleStorage implements Storage for S3-compatible backends
// (AWS S3, Cloudflare R2, MinIO, etc.).
type S3CompatibleStorage struct {
	endpoint string
	region   string
	bucket   string
	accessKey string
	secretKey string
	useSSL   bool
	// In production, use an S3 SDK client (e.g., aws-sdk-go-v2)
	// This is a scaffold ready for real SDK integration.
}

func NewS3CompatibleStorage(cfg *config.Config) *S3CompatibleStorage {
	return &S3CompatibleStorage{
		endpoint:  cfg.StorageS3Endpoint,
		region:    cfg.StorageS3Region,
		bucket:    cfg.StorageS3Bucket,
		accessKey: cfg.StorageS3AccessKey,
		secretKey: cfg.StorageS3SecretKey,
		useSSL:    cfg.StorageS3UseSSL,
	}
}

func (s *S3CompatibleStorage) getURL(key string) string {
	scheme := "https"
	if !s.useSSL {
		scheme = "http"
	}
	if s.endpoint != "" {
		return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, key)
	}
	return fmt.Sprintf("%s://%s.s3.%s.amazonaws.com/%s", scheme, s.bucket, s.region, key)
}

func (s *S3CompatibleStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (*FileInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}

	// Placeholder for S3 PutObject call:
	//   _, err := s3client.PutObject(ctx, &s3.PutObjectInput{
	//       Bucket: aws.String(s.bucket),
	//       Key:    aws.String(key),
	//       Body:   bytes.NewReader(data),
	//       ContentType: aws.String(contentType),
	//   })
	_ = bytes.NewReader(data)
	_ = contentType

	return &FileInfo{
		URL:         s.getURL(key),
		Bucket:      s.bucket,
		Key:         key,
		Size:        int64(len(data)),
		ContentType: contentType,
	}, nil
}

func (s *S3CompatibleStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	// Placeholder for S3 GetObject call.
	return io.NopCloser(bytes.NewReader(nil)), fmt.Errorf("S3 download not implemented")
}

func (s *S3CompatibleStorage) Delete(ctx context.Context, key string) error {
	// Placeholder for S3 DeleteObject call.
	return nil
}

func (s *S3CompatibleStorage) Exists(ctx context.Context, key string) (bool, error) {
	// Placeholder for S3 HeadObject call.
	return true, nil
}

func (s *S3CompatibleStorage) URL(ctx context.Context, key string) (string, error) {
	u, err := url.Parse(s.getURL(key))
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
