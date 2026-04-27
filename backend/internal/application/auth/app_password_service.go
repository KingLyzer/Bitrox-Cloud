package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
)

type AppPasswordRepository interface {
	Create(ctx context.Context, record identity.AppPassword) error
	Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
}

type AppPasswordService struct {
	repo     AppPasswordRepository
	tokenMgr TokenManager
	clock    Clock
}

func NewAppPasswordService(repo AppPasswordRepository, tokenMgr TokenManager, clock Clock) *AppPasswordService {
	return &AppPasswordService{
		repo:     repo,
		tokenMgr: tokenMgr,
		clock:    clock,
	}
}

type CreateAppPasswordInput struct {
	UserID    uuid.UUID
	Name      string
	ExpiresAt *time.Time
}

type CreateAppPasswordResult struct {
	ID        uuid.UUID
	Token     string
	CreatedAt time.Time
}

func (s *AppPasswordService) Create(ctx context.Context, input CreateAppPasswordInput) (CreateAppPasswordResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateAppPasswordResult{}, fmt.Errorf("name is required")
	}
	if len(name) > 120 {
		return CreateAppPasswordResult{}, fmt.Errorf("name is too long")
	}

	rawToken, tokenHash, err := s.tokenMgr.GenerateRefreshToken()
	if err != nil {
		return CreateAppPasswordResult{}, fmt.Errorf("generate app password token: %w", err)
	}

	now := s.clock.Now().UTC()
	record := identity.AppPassword{
		ID:        uuid.New(),
		UserID:    input.UserID,
		Name:      name,
		TokenHash: tokenHash,
		ExpiresAt: input.ExpiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return CreateAppPasswordResult{}, fmt.Errorf("create app password: %w", err)
	}

	return CreateAppPasswordResult{
		ID:        record.ID,
		Token:     rawToken,
		CreatedAt: now,
	}, nil
}

func (s *AppPasswordService) Revoke(ctx context.Context, appPasswordID uuid.UUID) error {
	return s.repo.Revoke(ctx, appPasswordID, s.clock.Now().UTC())
}
