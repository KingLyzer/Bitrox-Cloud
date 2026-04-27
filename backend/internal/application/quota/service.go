package quota

import (
	"context"
	"fmt"

	domainquota "cloud/backend/internal/domain/quota"

	"github.com/google/uuid"
)

type QuotaPolicyRepository interface {
	GetEffectiveUserQuota(ctx context.Context, userID uuid.UUID) (int64, error)
}

type UsageReader interface {
	GetUserUsedBytes(ctx context.Context, userID uuid.UUID) (int64, error)
}

type Service struct {
	policies QuotaPolicyRepository
	usage    UsageReader
}

func NewService(policies QuotaPolicyRepository, usage UsageReader) *Service {
	return &Service{policies: policies, usage: usage}
}

func (s *Service) GetUserUsage(ctx context.Context, userID uuid.UUID) (domainquota.Usage, error) {
	limit, err := s.policies.GetEffectiveUserQuota(ctx, userID)
	if err != nil {
		return domainquota.Usage{}, fmt.Errorf("get effective quota: %w", err)
	}

	used, err := s.usage.GetUserUsedBytes(ctx, userID)
	if err != nil {
		return domainquota.Usage{}, fmt.Errorf("get used bytes: %w", err)
	}

	remaining := limit - used
	exceeded := remaining < 0
	if remaining < 0 {
		remaining = 0
	}

	return domainquota.Usage{
		UserID:          userID,
		UsedBytes:       used,
		LimitBytes:      limit,
		RemainingBytes:  remaining,
		IsLimitExceeded: exceeded,
	}, nil
}

func (s *Service) EnforceWrite(ctx context.Context, userID uuid.UUID, additionalBytes int64) error {
	if additionalBytes < 0 {
		return fmt.Errorf("additional bytes cannot be negative")
	}

	usage, err := s.GetUserUsage(ctx, userID)
	if err != nil {
		return err
	}

	if usage.UsedBytes+additionalBytes > usage.LimitBytes {
		return fmt.Errorf("quota exceeded")
	}
	return nil
}

type StaticUsageReader struct{}

func NewStaticUsageReader() StaticUsageReader {
	return StaticUsageReader{}
}

func (StaticUsageReader) GetUserUsedBytes(context.Context, uuid.UUID) (int64, error) {
	// Phase 2 scaffold: real usage accounting is added in the file metadata phase.
	return 0, nil
}
