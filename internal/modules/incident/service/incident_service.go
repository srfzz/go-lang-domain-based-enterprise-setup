package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourorg/enterprise-api/internal/modules/incident/domain"
	"github.com/yourorg/enterprise-api/internal/modules/incident/dto"
	"github.com/yourorg/enterprise-api/internal/modules/incident/repository"
)

type IncidentService struct {
	repo *repository.IncidentRepository
}

func NewIncidentService(repo *repository.IncidentRepository) *IncidentService {
	return &IncidentService{repo: repo}
}

func (s *IncidentService) Create(ctx context.Context, req dto.CreateIncidentRequest, userID uuid.UUID) (*dto.IncidentResponse, error) {
	inc := domain.Incident{
		Title:       req.Title,
		Description: req.Description,
		Severity:    req.Severity,
		ReportedBy:  userID,
		Status:      "open",
	}
	if err := s.repo.Create(ctx, &inc); err != nil {
		return nil, err
	}
	return &dto.IncidentResponse{
		ID:          inc.ID.String(),
		Title:       inc.Title,
		Description: inc.Description,
		Status:      inc.Status,
		Severity:    inc.Severity,
		ReportedBy:  inc.ReportedBy.String(),
		CreatedAt:   inc.CreatedAt,
		UpdatedAt:   inc.UpdatedAt,
	}, nil
}

func (s *IncidentService) List(ctx context.Context, limit, offset int) ([]dto.IncidentResponse, int, error) {
	incidents, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var resp []dto.IncidentResponse
	for _, i := range incidents {
		resp = append(resp, dto.IncidentResponse{
			ID:          i.ID.String(),
			Title:       i.Title,
			Description: i.Description,
			Status:      i.Status,
			Severity:    i.Severity,
			ReportedBy:  i.ReportedBy.String(),
			CreatedAt:   i.CreatedAt,
			UpdatedAt:   i.UpdatedAt,
		})
	}
	return resp, total, nil
}
