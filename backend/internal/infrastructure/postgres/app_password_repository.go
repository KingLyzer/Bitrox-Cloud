package postgres

import (
	"context"
	"fmt"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppPasswordRepository struct {
	pool *pgxpool.Pool
}

func NewAppPasswordRepository(pool *pgxpool.Pool) *AppPasswordRepository {
	return &AppPasswordRepository{pool: pool}
}

func (r *AppPasswordRepository) Create(ctx context.Context, record identity.AppPassword) error {
	const query = `
INSERT INTO app_passwords (id, user_id, name, token_hash, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
`
	_, err := r.pool.Exec(ctx, query,
		record.ID,
		record.UserID,
		record.Name,
		record.TokenHash,
		record.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert app password: %w", err)
	}
	return nil
}

func (r *AppPasswordRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error {
	const query = `
UPDATE app_passwords
SET last_used_at = $2, updated_at = now()
WHERE id = $1
`
	if _, err := r.pool.Exec(ctx, query, id, lastUsedAt); err != nil {
		return fmt.Errorf("touch app password last_used_at: %w", err)
	}
	return nil
}

func (r *AppPasswordRepository) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	const query = `
UPDATE app_passwords
SET revoked_at = $2, updated_at = now()
WHERE id = $1
`
	if _, err := r.pool.Exec(ctx, query, id, revokedAt); err != nil {
		return fmt.Errorf("revoke app password: %w", err)
	}
	return nil
}
