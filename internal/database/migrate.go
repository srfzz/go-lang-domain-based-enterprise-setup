package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

func RunMigrations(pool *pgxpool.Pool) error {
	files, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, f := range files {
		data, err := migrationsFS.ReadFile(f)
		if err != nil {
			return err
		}
		_, err = pool.Exec(ctx, string(data))
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", f, err)
		}
	}
	return nil
}
