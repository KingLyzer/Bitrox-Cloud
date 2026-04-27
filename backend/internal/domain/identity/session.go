package identity

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	FamilyID            uuid.UUID
	RefreshTokenHash    string
	UserAgent           string
	IP                  string
	ExpiresAt           time.Time
	RevokedAt           *time.Time
	RevocationReason    *string
	ReplacedBySessionID *uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (s Session) IsRevoked() bool {
	return s.RevokedAt != nil
}
