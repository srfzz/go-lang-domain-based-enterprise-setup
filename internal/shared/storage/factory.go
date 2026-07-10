package storage

import (
	"fmt"

	"github.com/yourorg/enterprise-api/internal/config"
)

// NewFromConfig creates a Storage backend based on the config's STORAGE_DRIVER.
// Supported drivers: "local", "s3", "r2".
// R2 uses the same S3-compatible API, so it reuses S3CompatibleStorage.
func NewFromConfig(cfg *config.Config) (Storage, error) {
	switch cfg.StorageDriver {
	case "local":
		return NewLocalStorage(cfg.StorageLocalPath), nil
	case "s3", "r2":
		return NewS3CompatibleStorage(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.StorageDriver)
	}
}
