package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	authdomain "github.com/yourorg/enterprise-api/internal/modules/auth/domain"

	admindomain "github.com/yourorg/enterprise-api/internal/modules/admin/domain"
)

type AdminUserRepository struct {
	db *pgxpool.Pool
}

func NewAdminUserRepository(db *pgxpool.Pool) *AdminUserRepository {
	return &AdminUserRepository{db: db}
}

func (r *AdminUserRepository) Create(ctx context.Context, user *authdomain.User) error {
	query := `INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query, user.Email, user.PasswordHash, user.FullName).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *AdminUserRepository) FindByEmail(ctx context.Context, email string) (*authdomain.User, error) {
	u := &authdomain.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, is_active, created_at, updated_at FROM users WHERE email=$1`,
		email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *AdminUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*authdomain.User, error) {
	u := &authdomain.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, is_active, created_at, updated_at FROM users WHERE id=$1`,
		id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *AdminUserRepository) FindAll(ctx context.Context) ([]authdomain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT id, email, password_hash, full_name, is_active, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []authdomain.User
	for rows.Next() {
		var u authdomain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *AdminUserRepository) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID)
	return err
}

func (r *AdminUserRepository) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND role_id=$2`, userID, roleID)
	return err
}

func (r *AdminUserRepository) GetRoles(ctx context.Context, userID uuid.UUID) ([]admindomain.Role, error) {
	rows, err := r.db.Query(ctx,
		`SELECT r.id, r.name, r.category, r.description, r.created_at
		 FROM roles r JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []admindomain.Role
	for rows.Next() {
		var role admindomain.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Category, &role.Description, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *AdminUserRepository) SetRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
	if err != nil {
		return err
	}

	for _, rid := range roleIDs {
		_, err = tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, rid)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *AdminUserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET updated_at=$1 WHERE id=$2`, time.Now(), userID)
	return err
}
