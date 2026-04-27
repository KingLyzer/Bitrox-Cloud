package identity

import (
	"time"

	"github.com/google/uuid"
)

type AppPassword struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	TokenHash  string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
