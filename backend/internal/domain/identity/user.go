package identity

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID                uuid.UUID
	Email             string
	DisplayName       string
	PreferredLanguage *string
	Role              Role
	PasswordHash      string
	QuotaBytes        *int64
	IsActive          bool
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateUserInput struct {
	Email             string
	DisplayName       string
	PreferredLanguage *string
	Role              Role
	PasswordHash      string
	QuotaBytes        *int64
	IsActive          bool
}

type UpdateUserInput struct {
	ID          uuid.UUID
	DisplayName string
	Role        Role
	QuotaBytes  *int64
	IsActive    bool
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func IsValidRole(role Role) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}
