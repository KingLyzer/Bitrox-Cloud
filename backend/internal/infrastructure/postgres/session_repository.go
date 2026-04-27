package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, session identity.Session) error {
	const query = `
INSERT INTO sessions (
	id, user_id, family_id, refresh_token_hash, user_agent, ip, expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
`
	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.FamilyID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IP,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (identity.Session, error) {
	const query = `
SELECT id, user_id, family_id, refresh_token_hash, user_agent, ip, expires_at,
       revoked_at, revocation_reason, replaced_by_session_id, created_at, updated_at
FROM sessions
WHERE refresh_token_hash = $1
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, refreshTokenHash)
	session, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.Session{}, identity.ErrSessionNotFound
		}
		return identity.Session{}, fmt.Errorf("query session by token hash: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (identity.Session, error) {
	const query = `
SELECT id, user_id, family_id, refresh_token_hash, user_agent, ip, expires_at,
       revoked_at, revocation_reason, replaced_by_session_id, created_at, updated_at
FROM sessions
WHERE id = $1
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, id)
	session, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.Session{}, identity.ErrSessionNotFound
		}
		return identity.Session{}, fmt.Errorf("query session by id: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) Rotate(ctx context.Context, oldSessionID uuid.UUID, newSession identity.Session, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insertNew = `
INSERT INTO sessions (
	id, user_id, family_id, refresh_token_hash, user_agent, ip, expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
`
	_, err = tx.Exec(ctx, insertNew,
		newSession.ID,
		newSession.UserID,
		newSession.FamilyID,
		newSession.RefreshTokenHash,
		newSession.UserAgent,
		newSession.IP,
		newSession.ExpiresAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return identity.ErrSessionAlreadyRotated
		}
		return fmt.Errorf("insert rotated session: %w", err)
	}

	const updateOld = `
UPDATE sessions
SET revoked_at = $2, revocation_reason = 'rotated', replaced_by_session_id = $3, updated_at = now()
WHERE id = $1
  AND revoked_at IS NULL
`
	tag, err := tx.Exec(ctx, updateOld, oldSessionID, now, newSession.ID)
	if err != nil {
		return fmt.Errorf("update old session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrSessionAlreadyRotated
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rotate transaction: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeByRefreshTokenHash(ctx context.Context, refreshTokenHash string, now time.Time, reason string) (bool, error) {
	const query = `
UPDATE sessions
SET revoked_at = $2, revocation_reason = $3, updated_at = now()
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, refreshTokenHash, now, reason)
	if err != nil {
		return false, fmt.Errorf("revoke session by refresh token hash: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID uuid.UUID, now time.Time, reason string) (bool, error) {
	const query = `
UPDATE sessions
SET revoked_at = $2, revocation_reason = $3, updated_at = now()
WHERE id = $1
  AND revoked_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, sessionID, now, reason)
	if err != nil {
		return false, fmt.Errorf("revoke session by id: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *SessionRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time, reason string) error {
	const query = `
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, $2),
    revocation_reason = COALESCE(revocation_reason, $3),
    updated_at = now()
WHERE family_id = $1
`
	if _, err := r.pool.Exec(ctx, query, familyID, now, reason); err != nil {
		return fmt.Errorf("revoke session family: %w", err)
	}
	return nil
}

type sessionRow interface {
	Scan(dest ...any) error
}

func scanSession(row sessionRow) (identity.Session, error) {
	var s identity.Session
	var revokedAt sql.NullTime
	var revocationReason sql.NullString
	var replacedBySessionID sql.NullString
	if err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.FamilyID,
		&s.RefreshTokenHash,
		&s.UserAgent,
		&s.IP,
		&s.ExpiresAt,
		&revokedAt,
		&revocationReason,
		&replacedBySessionID,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return identity.Session{}, err
	}
	if revokedAt.Valid {
		v := revokedAt.Time
		s.RevokedAt = &v
	}
	if revocationReason.Valid {
		v := revocationReason.String
		s.RevocationReason = &v
	}
	if replacedBySessionID.Valid {
		parsed, err := uuid.Parse(replacedBySessionID.String)
		if err != nil {
			return identity.Session{}, fmt.Errorf("parse replaced_by_session_id: %w", err)
		}
		s.ReplacedBySessionID = &parsed
	}
	return s, nil
}
