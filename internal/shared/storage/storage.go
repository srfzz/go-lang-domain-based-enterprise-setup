package storage

import (
	"context"
	"io"
)

// FileInfo contains metadata about a stored file.
type FileInfo struct {
	URL          string `json:"url"`
	Bucket       string `json:"bucket"`
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	ContentType  string `json:"content_type"`
}

// Storage defines the interface for file storage backends.
// Implementations: LocalStorage, S3Storage, R2Storage, etc.
type Storage interface {
	// Upload stores a file and returns its metadata.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (*FileInfo, error)

	// Download retrieves a file by key.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a file by key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists.
	Exists(ctx context.Context, key string) (bool, error)

	// URL returns the public URL for a file, if applicable.
	URL(ctx context.Context, key string) (string, error)
}
