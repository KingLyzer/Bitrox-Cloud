package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud/backend/internal/domain/identity"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (identity.User, error)
	Create(ctx context.Context, input identity.CreateUserInput) (identity.User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type AdminSeedInput struct {
	Email      string
	Password   string
	DisplayName string
	Role       identity.Role
	QuotaBytes *int64
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

	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil
	}
	if !errors.Is(err, identity.ErrUserNotFound) {
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

	_, err = s.users.Create(ctx, identity.CreateUserInput{
		Email:        email,
		DisplayName:  displayName,
		Role:         role,
		PasswordHash: passwordHash,
		QuotaBytes:   input.QuotaBytes,
		IsActive:     true,
	})
	if err != nil {
		return fmt.Errorf("create bootstrap admin: %w", err)
	}

	return nil
}
