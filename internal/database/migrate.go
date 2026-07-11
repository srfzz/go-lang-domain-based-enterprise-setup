package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

type Migration struct {
	Version   string
	Name      string
	AppliedAt time.Time
}

func RunMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// 1. Create migrations table if it doesn't exist
	if err := createMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// 2. Get already applied migrations
	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// 3. Get migration files
	files, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	// 4. Run pending migrations
	for _, f := range files {
		version := extractVersion(f)

		// Skip if already applied
		if applied[version] {
			fmt.Printf("⏭️ Migration %s already applied, skipping\n", version)
			continue
		}

		fmt.Printf("🔄 Running migration: %s\n", version)

		// Read migration file
		data, err := migrationsFS.ReadFile(f)
		if err != nil {
			return err
		}

		sql := string(data)

		// Fix CREATE TABLE statements to use IF NOT EXISTS
		sql = fixCreateTableStatements(sql)

		// Run migration in transaction
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}

		// Execute migration
		_, err = tx.Exec(ctx, sql)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", f, err)
		}

		// Record migration
		_, err = tx.Exec(ctx, `
			INSERT INTO schema_migrations (version, name, applied_at) 
			VALUES ($1, $2, CURRENT_TIMESTAMP)
		`, version, getMigrationName(f))
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		fmt.Printf("✅ Migration %s completed\n", version)
	}

	return nil
}

func createMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            id SERIAL PRIMARY KEY,
            version VARCHAR(255) NOT NULL UNIQUE,
            name VARCHAR(255) NOT NULL,
            applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
	return err
}

func getAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

func extractVersion(path string) string {
	// migrations/000001_create_users.up.sql -> 000001
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "unknown"
	}
	filename := parts[len(parts)-1]
	return strings.TrimSuffix(filename, ".up.sql")
}

func getMigrationName(path string) string {
	// migrations/000001_create_users.up.sql -> create_users
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "unknown"
	}
	filename := parts[len(parts)-1]
	name := strings.TrimSuffix(filename, ".up.sql")
	// Remove version prefix: 000001_create_users -> create_users
	parts = strings.SplitN(name, "_", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}

func fixCreateTableStatements(sql string) string {
	// Split into individual statements
	statements := strings.Split(sql, ";")

	for i, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}

		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "CREATE TABLE") && !strings.Contains(upper, "IF NOT EXISTS") {
			statements[i] = strings.Replace(trimmed, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS", 1)
		}
	}

	return strings.Join(statements, ";")
}
