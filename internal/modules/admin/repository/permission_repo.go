package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/modules/admin/domain"
)

type PermissionRepository struct {
	db *pgxpool.Pool
}

func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) Create(ctx context.Context, p *domain.Permission) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO permissions (name, action, resource, description) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		p.Name, p.Action, p.Resource, p.Description,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *PermissionRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.Permission, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := fmt.Sprintf(`SELECT id, name, action, resource, description, created_at FROM permissions ORDER BY resource, action LIMIT %d OFFSET %d`, limit, offset)
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Action, &p.Resource, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (r *PermissionRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM permissions`).Scan(&count)
	return count, err
}

func (r *PermissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	p := &domain.Permission{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, action, resource, description, created_at FROM permissions WHERE id=$1`, id,
	).Scan(&p.ID, &p.Name, &p.Action, &p.Resource, &p.Description, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PermissionRepository) Update(ctx context.Context, id uuid.UUID, p *domain.Permission) error {
	_, err := r.db.Exec(ctx,
		`UPDATE permissions SET name=$1, action=$2, resource=$3, description=$4 WHERE id=$5`,
		p.Name, p.Action, p.Resource, p.Description, id,
	)
	return err
}

func (r *PermissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM permissions WHERE id=$1`, id)
	return err
}
