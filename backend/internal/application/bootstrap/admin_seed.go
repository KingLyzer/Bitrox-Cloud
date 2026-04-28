package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud/backend/internal/domain/identity"
	"github.com/google/uuid"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (identity.User, error)
	Create(ctx context.Context, input identity.CreateUserInput) (identity.User, error)
	Update(ctx context.Context, input identity.UpdateUserInput) (identity.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, displayName string, preferredLanguage *string) (identity.User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type AdminSeedInput struct {
	Email             string
	Password          string
	DisplayName       string
	PreferredLanguage *string
	Role              identity.Role
	QuotaBytes        *int64
}

type AdminSeeder struct {
	users  UserRepository
	hasher PasswordHasher
}

func NewAdminSeeder(users UserRepository, hasher PasswordHasher) *AdminSeeder {
	return &AdminSeeder{users: users, hasher: hasher}
}

func (s *AdminSeeder) Ensure(ctx context.Context, input AdminSeedInput) error {
	email := identity.NormalizeEmail(input.Email)
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" {
		return nil
	}

	if len(password) < 12 {
		return fmt.Errorf("bootstrap admin password must be at least 12 characters")
	}

	role := input.Role
	if role == "" {
		role = identity.RoleOwner
	}
	if !identity.IsValidRole(role) {
		return fmt.Errorf("invalid bootstrap admin role: %s", role)
	}

	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, identity.ErrUserNotFound) {
		return fmt.Errorf("lookup bootstrap admin: %w", err)
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = "Admin"
	}
	preferredLanguage := input.PreferredLanguage
	if preferredLanguage == nil {
		defaultLang := "en"
		preferredLanguage = &defaultLang
	}

	if errors.Is(err, identity.ErrUserNotFound) {
		_, createErr := s.users.Create(ctx, identity.CreateUserInput{
			Email:             email,
			DisplayName:       displayName,
			PreferredLanguage: preferredLanguage,
			Role:              role,
			PasswordHash:      passwordHash,
			QuotaBytes:        input.QuotaBytes,
			IsActive:          true,
		})
		if createErr != nil {
			return fmt.Errorf("create bootstrap admin: %w", createErr)
		}
		return nil
	}

	needsIdentityUpdate := !existing.IsActive || existing.Role != role || strings.TrimSpace(existing.DisplayName) == ""
	if needsIdentityUpdate {
		targetDisplayName := strings.TrimSpace(existing.DisplayName)
		if targetDisplayName == "" {
			targetDisplayName = displayName
		}
		if _, updateErr := s.users.Update(ctx, identity.UpdateUserInput{
			ID:          existing.ID,
			DisplayName: targetDisplayName,
			Role:        role,
			QuotaBytes:  existing.QuotaBytes,
			IsActive:    true,
		}); updateErr != nil {
			return fmt.Errorf("update bootstrap admin identity: %w", updateErr)
		}
	}

	if existing.PreferredLanguage == nil || strings.TrimSpace(*existing.PreferredLanguage) == "" {
		targetDisplayName := strings.TrimSpace(existing.DisplayName)
		if targetDisplayName == "" {
			targetDisplayName = displayName
		}
		if _, profileErr := s.users.UpdateProfile(ctx, existing.ID, targetDisplayName, preferredLanguage); profileErr != nil {
			return fmt.Errorf("update bootstrap admin profile: %w", profileErr)
		}
	}

	return nil
}
