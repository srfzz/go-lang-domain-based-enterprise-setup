package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	abs, _ := filepath.Abs(basePath)
	os.MkdirAll(abs, 0755)
	return &LocalStorage{basePath: abs}
}

func (s *LocalStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (*FileInfo, error) {
	fullPath := filepath.Join(s.basePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}
	defer dst.Close()

	size, err := io.Copy(dst, reader)
	if err != nil {
		return nil, fmt.Errorf("writing file: %w", err)
	}

	return &FileInfo{
		URL:         fullPath,
		Key:         key,
		Size:        size,
		ContentType: contentType,
	}, nil
}

func (s *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, key)
	return os.Open(fullPath)
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	fullPath := filepath.Join(s.basePath, key)
	return os.Remove(fullPath)
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	fullPath := filepath.Join(s.basePath, key)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (s *LocalStorage) URL(ctx context.Context, key string) (string, error) {
	fullPath := filepath.Join(s.basePath, key)
	return fullPath, nil
}
