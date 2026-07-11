package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/modules/incident/domain"
)

type IncidentRepository struct {
	db *pgxpool.Pool
}

func NewIncidentRepository(db *pgxpool.Pool) *IncidentRepository {
	return &IncidentRepository{db: db}
}

func (r *IncidentRepository) Create(ctx context.Context, inc *domain.Incident) error {
	query := `INSERT INTO incidents (title, description, severity, reported_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query, inc.Title, inc.Description, inc.Severity, inc.ReportedBy).
		Scan(&inc.ID, &inc.CreatedAt, &inc.UpdatedAt)
}

func (r *IncidentRepository) List(ctx context.Context, limit, offset int) ([]domain.Incident, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := fmt.Sprintf(
		`SELECT id, title, description, status, severity, reported_by, created_at, updated_at FROM incidents ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		limit, offset)
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var incidents []domain.Incident
	for rows.Next() {
		var i domain.Incident
		if err := rows.Scan(&i.ID, &i.Title, &i.Description, &i.Status, &i.Severity, &i.ReportedBy, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		incidents = append(incidents, i)
	}
	return incidents, nil
}

func (r *IncidentRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM incidents`).Scan(&count)
	return count, err
}
