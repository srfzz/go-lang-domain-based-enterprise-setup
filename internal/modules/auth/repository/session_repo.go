package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/modules/auth/domain"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	query := `INSERT INTO sessions (user_id, refresh_token_hash, device_id, ip_address, user_agent)
	           VALUES ($1, $2, $3, $4::inet, $5) RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		session.UserID, session.RefreshTokenHash, session.DeviceID, session.IPAddress, session.UserAgent,
	).Scan(&session.ID, &session.CreatedAt)
}

func (r *SessionRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func (r *SessionRepository) DeleteByRefreshHash(ctx context.Context, hash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE refresh_token_hash = $1`, hash)
	return err
}

func (r *SessionRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func (r *SessionRepository) FindOldestByUserID(ctx context.Context, userID uuid.UUID) (*domain.Session, error) {
	s := &domain.Session{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, refresh_token_hash, device_id, ip_address, user_agent, last_accessed_at, created_at
		 FROM sessions WHERE user_id = $1 ORDER BY last_accessed_at ASC LIMIT 1`, userID,
	).Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.DeviceID, &s.IPAddress, &s.UserAgent, &s.LastAccessedAt, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *SessionRepository) UpdateLastAccessed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET last_accessed_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}
