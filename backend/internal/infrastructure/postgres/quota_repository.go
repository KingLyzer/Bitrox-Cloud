package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuotaRepository struct {
	pool *pgxpool.Pool
}

func NewQuotaRepository(pool *pgxpool.Pool) *QuotaRepository {
	return &QuotaRepository{pool: pool}
}

func (r *QuotaRepository) GetEffectiveUserQuota(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
SELECT COALESCE(u.quota_bytes, qp.quota_bytes) AS effective_quota
FROM users u
JOIN quota_policies qp ON qp.key = 'default_user_quota'
WHERE u.id = $1
LIMIT 1
`
	var quotaBytes int64
	if err := r.pool.QueryRow(ctx, query, userID).Scan(&quotaBytes); err != nil {
		return 0, fmt.Errorf("query effective user quota: %w", err)
	}
	return quotaBytes, nil
}

func (r *QuotaRepository) UpsertDefaultUserQuota(ctx context.Context, quotaBytes int64) error {
	const query = `
INSERT INTO quota_policies (key, quota_bytes, created_at, updated_at)
VALUES ('default_user_quota', $1, now(), now())
ON CONFLICT (key)
DO UPDATE SET quota_bytes = EXCLUDED.quota_bytes, updated_at = now()
`
	if _, err := r.pool.Exec(ctx, query, quotaBytes); err != nil {
		return fmt.Errorf("upsert default user quota policy: %w", err)
	}
	return nil
}
