package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

type AppSetting struct {
	Key             string
	ValueJSON       json.RawMessage
	UpdatedByUserID *uuid.UUID
	UpdatedAt       time.Time
}

func (r *SettingsRepository) List(ctx context.Context) ([]AppSetting, error) {
	const query = `
SELECT key, value_json, updated_by_user_id, updated_at
FROM app_settings
ORDER BY key ASC
`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query app settings: %w", err)
	}
	defer rows.Close()

	out := make([]AppSetting, 0, 8)
	for rows.Next() {
		var (
			item      AppSetting
			updatedBy *uuid.UUID
		)
		if err := rows.Scan(&item.Key, &item.ValueJSON, &updatedBy, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan app setting: %w", err)
		}
		item.UpdatedByUserID = updatedBy
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app settings: %w", err)
	}
	return out, nil
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (AppSetting, error) {
	const query = `
SELECT key, value_json, updated_by_user_id, updated_at
FROM app_settings
WHERE key = $1
LIMIT 1
`
	var (
		item      AppSetting
		updatedBy *uuid.UUID
	)
	if err := r.pool.QueryRow(ctx, query, key).Scan(&item.Key, &item.ValueJSON, &updatedBy, &item.UpdatedAt); err != nil {
		return AppSetting{}, fmt.Errorf("get app setting %s: %w", key, err)
	}
	item.UpdatedByUserID = updatedBy
	return item, nil
}

func (r *SettingsRepository) Upsert(ctx context.Context, key string, value any, updatedBy *uuid.UUID) (AppSetting, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return AppSetting{}, fmt.Errorf("marshal app setting %s: %w", key, err)
	}

	const query = `
INSERT INTO app_settings (key, value_json, updated_by_user_id, updated_at)
VALUES ($1, $2::jsonb, $3, now())
ON CONFLICT (key) DO UPDATE
SET value_json = EXCLUDED.value_json,
    updated_by_user_id = EXCLUDED.updated_by_user_id,
    updated_at = now()
RETURNING key, value_json, updated_by_user_id, updated_at
`
	var (
		item      AppSetting
		updatedByUser *uuid.UUID
	)
	if err := r.pool.QueryRow(ctx, query, key, string(raw), updatedBy).Scan(&item.Key, &item.ValueJSON, &updatedByUser, &item.UpdatedAt); err != nil {
		return AppSetting{}, fmt.Errorf("upsert app setting %s: %w", key, err)
	}
	item.UpdatedByUserID = updatedByUser
	return item, nil
}
