package domain

import (
	"time"
	"github.com/google/uuid"
)

type Incident struct {
	ID          uuid.UUID
	Title       string
	Description string
	Status      string
	Severity    string
	ReportedBy  uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
