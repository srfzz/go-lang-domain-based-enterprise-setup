package domain

import (
	"time"
	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	DeviceID  string
	IPAddress string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	DeviceID         string
	IPAddress        string
	UserAgent        string
	LastAccessedAt   time.Time
	CreatedAt        time.Time
}
