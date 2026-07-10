package domain

import (
	"time"
	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID
	Name        string
	Category    string
	Description string
	CreatedAt   time.Time
}

type Permission struct {
	ID          uuid.UUID
	Name        string
	Action      string
	Resource    string
	Description string
	CreatedAt   time.Time
}

type RolePermission struct {
	RoleID       uuid.UUID
	PermissionID uuid.UUID
}
