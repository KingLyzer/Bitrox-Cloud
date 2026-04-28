package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (identity.User, error) {
	const query = `
SELECT id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
FROM users
WHERE lower(email) = lower($1)
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, email)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("query user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (identity.User, error) {
	const query = `
SELECT id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, id)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("query user by id: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, input identity.CreateUserInput) (identity.User, error) {
	const query = `
INSERT INTO users (email, display_name, preferred_language, role, password_hash, quota_bytes, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
`
	row := r.pool.QueryRow(ctx, query,
		identity.NormalizeEmail(input.Email),
		input.DisplayName,
		input.PreferredLanguage,
		string(input.Role),
		input.PasswordHash,
		input.QuotaBytes,
		input.IsActive,
	)
	user, err := scanUser(row)
	if err != nil {
		return identity.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]identity.User, error) {
	const query = `
SELECT id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
FROM users
ORDER BY created_at ASC
`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]identity.User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan listed user: %w", scanErr)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, input identity.UpdateUserInput) (identity.User, error) {
	const query = `
UPDATE users
SET
	display_name = $2,
	role = $3,
	quota_bytes = $4,
	is_active = $5,
	updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
`

	row := r.pool.QueryRow(
		ctx,
		query,
		input.ID,
		strings.TrimSpace(input.DisplayName),
		string(input.Role),
		input.QuotaBytes,
		input.IsActive,
	)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (identity.User, error) {
	const query = `
UPDATE users
SET
	password_hash = $2,
	updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
`

	row := r.pool.QueryRow(ctx, query, userID, strings.TrimSpace(passwordHash))
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("update user password hash: %w", err)
	}
	return user, nil
}

func (r *UserRepository) SoftDeleteByID(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `
UPDATE users
SET
	is_active = FALSE,
	deleted_at = COALESCE(deleted_at, now()),
	updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return false, fmt.Errorf("soft delete user: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *UserRepository) HardDeleteByID(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `
DELETE FROM users
WHERE id = $1
  AND deleted_at IS NOT NULL
`
	tag, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return false, fmt.Errorf("hard delete user: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName string, preferredLanguage *string) (identity.User, error) {
	const query = `
UPDATE users
SET
	display_name = $2,
	preferred_language = $3,
	updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
`
	row := r.pool.QueryRow(ctx, query, userID, strings.TrimSpace(displayName), preferredLanguage)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("update user profile: %w", err)
	}
	return user, nil
}

func (r *UserRepository) ReactivateByID(
	ctx context.Context,
	input identity.UpdateUserInput,
	passwordHash *string,
) (identity.User, error) {
	const query = `
UPDATE users
SET
	display_name = $2,
	role = $3,
	quota_bytes = $4,
	is_active = TRUE,
	deleted_at = NULL,
	password_hash = COALESCE($5, password_hash),
	updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, preferred_language, role, password_hash, quota_bytes, is_active, deleted_at, created_at, updated_at
`
	row := r.pool.QueryRow(
		ctx,
		query,
		input.ID,
		strings.TrimSpace(input.DisplayName),
		string(input.Role),
		input.QuotaBytes,
		passwordHash,
	)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("reactivate user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) CountByRole(ctx context.Context, role identity.Role) (int, error) {
	const query = `
SELECT COUNT(*)
FROM users
WHERE role = $1
  AND deleted_at IS NULL
`
	var count int
	if err := r.pool.QueryRow(ctx, query, string(role)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return count, nil
}

func (r *UserRepository) CountActiveByRole(ctx context.Context, role identity.Role) (int, error) {
	const query = `
SELECT COUNT(*)
FROM users
WHERE role = $1
  AND is_active = TRUE
  AND deleted_at IS NULL
`
	var count int
	if err := r.pool.QueryRow(ctx, query, string(role)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active users by role: %w", err)
	}
	return count, nil
}

type userRow interface {
	Scan(dest ...any) error
}

func scanUser(row userRow) (identity.User, error) {
	var (
		u         identity.User
		quotaRaw  sql.NullInt64
		deletedAt sql.NullTime
		roleText  string
	)

	if err := row.Scan(
		&u.ID,
		&u.Email,
		&u.DisplayName,
		&u.PreferredLanguage,
		&roleText,
		&u.PasswordHash,
		&quotaRaw,
		&u.IsActive,
		&deletedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	); err != nil {
		return identity.User{}, err
	}

	u.Role = identity.Role(roleText)
	if quotaRaw.Valid {
		v := quotaRaw.Int64
		u.QuotaBytes = &v
	}
	if deletedAt.Valid {
		v := deletedAt.Time
		u.DeletedAt = &v
	}

	return u, nil
}
