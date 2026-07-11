package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/modules/admin/domain"
)

type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, role *domain.Role) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO roles (name, category, description) VALUES ($1, $2, $3) RETURNING id, created_at`,
		role.Name, role.Category, role.Description,
	).Scan(&role.ID, &role.CreatedAt)
}

func (r *RoleRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.Role, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := fmt.Sprintf(`SELECT id, name, category, description, created_at FROM roles ORDER BY name LIMIT %d OFFSET %d`, limit, offset)
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Category, &role.Description, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *RoleRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM roles`).Scan(&count)
	return count, err
}

func (r *RoleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	role := &domain.Role{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, category, description, created_at FROM roles WHERE id=$1`, id,
	).Scan(&role.ID, &role.Name, &role.Category, &role.Description, &role.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return role, err
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	role := &domain.Role{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, category, description, created_at FROM roles WHERE name=$1`, name,
	).Scan(&role.ID, &role.Name, &role.Category, &role.Description, &role.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return role, err
}

func (r *RoleRepository) Update(ctx context.Context, id uuid.UUID, role *domain.Role) error {
	_, err := r.db.Exec(ctx,
		`UPDATE roles SET name=$1, category=$2, description=$3 WHERE id=$4`,
		role.Name, role.Category, role.Description, id,
	)
	return err
}

func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM roles WHERE id=$1`, id)
	return err
}

func (r *RoleRepository) AssignPermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
	if err != nil {
		return err
	}

	for _, pid := range permissionIDs {
		_, err = tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, pid)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RoleRepository) GetPermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.name, p.action, p.resource, p.description, p.created_at
		 FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 WHERE rp.role_id = $1`, roleID)
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
