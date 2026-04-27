package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShareRepository struct {
	pool *pgxpool.Pool
}

func NewShareRepository(pool *pgxpool.Pool) *ShareRepository {
	return &ShareRepository{pool: pool}
}

type ShareRecord struct {
	ID            uuid.UUID
	NodeID        uuid.UUID
	OwnerUserID   uuid.UUID
	TokenHash     string
	PasswordHash  *string
	ExpiresAt     *time.Time
	MaxDownloads  *int
	DownloadCount int
	AllowDownload bool
	AllowPreview  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevokedAt     *time.Time
}

type ShareInput struct {
	NodeID        uuid.UUID
	OwnerUserID   uuid.UUID
	TokenHash     string
	PasswordHash  *string
	ExpiresAt     *time.Time
	MaxDownloads  *int
	AllowDownload bool
	AllowPreview  bool
}

type PublicShareRecord struct {
	Share ShareRecord
	Node  FilesNodeSummary
}

type FilesNodeSummary struct {
	ID          uuid.UUID
	OwnerUserID uuid.UUID
	Type        string
	Name        string
	SizeBytes   int64
	MIMEType    *string
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (r *ShareRepository) Create(ctx context.Context, input ShareInput) (ShareRecord, error) {
	const query = `
INSERT INTO shares (
    node_id, owner_user_id, token_hash, password_hash, expires_at, max_downloads,
    allow_download, allow_preview, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
RETURNING id, node_id, owner_user_id, token_hash, password_hash, expires_at,
          max_downloads, download_count, allow_download, allow_preview,
          created_at, updated_at, revoked_at
`
	row := r.pool.QueryRow(ctx, query,
		input.NodeID,
		input.OwnerUserID,
		strings.TrimSpace(input.TokenHash),
		input.PasswordHash,
		input.ExpiresAt,
		input.MaxDownloads,
		input.AllowDownload,
		input.AllowPreview,
	)
	share, err := scanShare(row)
	if err != nil {
		return ShareRecord{}, fmt.Errorf("create share: %w", err)
	}
	return share, nil
}

func (r *ShareRepository) ListByOwner(ctx context.Context, ownerUserID uuid.UUID, nodeID *uuid.UUID) ([]ShareRecord, error) {
	query := `
SELECT id, node_id, owner_user_id, token_hash, password_hash, expires_at,
       max_downloads, download_count, allow_download, allow_preview,
       created_at, updated_at, revoked_at
FROM shares
WHERE owner_user_id = $1
`
	args := []any{ownerUserID}
	if nodeID != nil {
		query += " AND node_id = $2\n"
		args = append(args, *nodeID)
	}
	query += "ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	out := make([]ShareRecord, 0)
	for rows.Next() {
		item, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("scan share: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shares: %w", err)
	}
	return out, nil
}

func (r *ShareRepository) GetByIDForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (ShareRecord, error) {
	const query = `
SELECT id, node_id, owner_user_id, token_hash, password_hash, expires_at,
       max_downloads, download_count, allow_download, allow_preview,
       created_at, updated_at, revoked_at
FROM shares
WHERE owner_user_id = $1
  AND id = $2
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, ownerUserID, shareID)
	item, err := scanShare(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShareRecord{}, pgx.ErrNoRows
		}
		return ShareRecord{}, fmt.Errorf("get share by id: %w", err)
	}
	return item, nil
}

func (r *ShareRepository) UpdateForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID, expiresAt *time.Time, maxDownloads *int, allowDownload *bool, passwordHash *string, clearPassword bool) (ShareRecord, error) {
	const query = `
UPDATE shares
SET expires_at = COALESCE($3, expires_at),
    max_downloads = COALESCE($4, max_downloads),
    allow_download = COALESCE($5, allow_download),
    password_hash = CASE
        WHEN $7 = TRUE THEN NULL
        WHEN $6 IS NOT NULL THEN $6
        ELSE password_hash
    END,
    updated_at = now()
WHERE owner_user_id = $1
  AND id = $2
RETURNING id, node_id, owner_user_id, token_hash, password_hash, expires_at,
          max_downloads, download_count, allow_download, allow_preview,
          created_at, updated_at, revoked_at
`
	row := r.pool.QueryRow(ctx, query, ownerUserID, shareID, expiresAt, maxDownloads, allowDownload, passwordHash, clearPassword)
	item, err := scanShare(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShareRecord{}, pgx.ErrNoRows
		}
		return ShareRecord{}, fmt.Errorf("update share: %w", err)
	}
	return item, nil
}

func (r *ShareRepository) RevokeForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (bool, error) {
	const query = `
UPDATE shares
SET revoked_at = COALESCE(revoked_at, now()),
    updated_at = now()
WHERE owner_user_id = $1
  AND id = $2
  AND revoked_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, ownerUserID, shareID)
	if err != nil {
		return false, fmt.Errorf("revoke share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ShareRepository) GetPublicByTokenHash(ctx context.Context, tokenHash string) (PublicShareRecord, error) {
	const query = `
SELECT
    s.id, s.node_id, s.owner_user_id, s.token_hash, s.password_hash, s.expires_at,
    s.max_downloads, s.download_count, s.allow_download, s.allow_preview,
    s.created_at, s.updated_at, s.revoked_at,
    n.id, n.owner_user_id, n.type, n.name, n.size_bytes, n.mime_type, n.updated_at, n.deleted_at
FROM shares s
JOIN nodes n ON n.id = s.node_id
WHERE s.token_hash = $1
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, strings.TrimSpace(tokenHash))
	var (
		record PublicShareRecord
		node   FilesNodeSummary
	)
	share, err := scanShareWithNode(row, &node)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicShareRecord{}, pgx.ErrNoRows
		}
		return PublicShareRecord{}, fmt.Errorf("get public share by token hash: %w", err)
	}
	record.Share = share
	record.Node = node
	return record, nil
}

func (r *ShareRepository) IncrementDownloadCount(ctx context.Context, shareID uuid.UUID) (ShareRecord, error) {
	const query = `
UPDATE shares
SET download_count = download_count + 1,
    updated_at = now()
WHERE id = $1
RETURNING id, node_id, owner_user_id, token_hash, password_hash, expires_at,
          max_downloads, download_count, allow_download, allow_preview,
          created_at, updated_at, revoked_at
`
	row := r.pool.QueryRow(ctx, query, shareID)
	share, err := scanShare(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShareRecord{}, pgx.ErrNoRows
		}
		return ShareRecord{}, fmt.Errorf("increment share download count: %w", err)
	}
	return share, nil
}

func (r *ShareRepository) CreateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, expiresAt time.Time) error {
	const query = `
INSERT INTO share_access_grants (share_id, grant_hash, expires_at, created_at)
VALUES ($1, $2, $3, now())
`
	if _, err := r.pool.Exec(ctx, query, shareID, strings.TrimSpace(grantHash), expiresAt); err != nil {
		return fmt.Errorf("create share access grant: %w", err)
	}
	return nil
}

func (r *ShareRepository) ValidateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, now time.Time) (bool, error) {
	const query = `
SELECT EXISTS (
    SELECT 1
    FROM share_access_grants
    WHERE share_id = $1
      AND grant_hash = $2
      AND expires_at > $3
)
`
	var ok bool
	if err := r.pool.QueryRow(ctx, query, shareID, strings.TrimSpace(grantHash), now).Scan(&ok); err != nil {
		return false, fmt.Errorf("validate share access grant: %w", err)
	}
	return ok, nil
}

func (r *ShareRepository) GetNodeSummaryForOwner(ctx context.Context, ownerUserID, nodeID uuid.UUID) (FilesNodeSummary, error) {
	const query = `
SELECT id, owner_user_id, type, name, size_bytes, mime_type, updated_at, deleted_at
FROM nodes
WHERE owner_user_id = $1
  AND id = $2
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, ownerUserID, nodeID)
	node, err := scanNodeSummary(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FilesNodeSummary{}, pgx.ErrNoRows
		}
		return FilesNodeSummary{}, fmt.Errorf("get node summary for share owner: %w", err)
	}
	return node, nil
}

type shareScanner interface {
	Scan(dest ...any) error
}

func scanShare(row shareScanner) (ShareRecord, error) {
	var (
		item          ShareRecord
		passwordRaw   sql.NullString
		expiresRaw    sql.NullTime
		maxRaw        sql.NullInt32
		revokedRaw    sql.NullTime
	)
	if err := row.Scan(
		&item.ID,
		&item.NodeID,
		&item.OwnerUserID,
		&item.TokenHash,
		&passwordRaw,
		&expiresRaw,
		&maxRaw,
		&item.DownloadCount,
		&item.AllowDownload,
		&item.AllowPreview,
		&item.CreatedAt,
		&item.UpdatedAt,
		&revokedRaw,
	); err != nil {
		return ShareRecord{}, err
	}
	if passwordRaw.Valid {
		value := passwordRaw.String
		item.PasswordHash = &value
	}
	if expiresRaw.Valid {
		value := expiresRaw.Time
		item.ExpiresAt = &value
	}
	if maxRaw.Valid {
		value := int(maxRaw.Int32)
		item.MaxDownloads = &value
	}
	if revokedRaw.Valid {
		value := revokedRaw.Time
		item.RevokedAt = &value
	}
	return item, nil
}

func scanNodeSummary(row shareScanner) (FilesNodeSummary, error) {
	var (
		node       FilesNodeSummary
		mimeRaw    sql.NullString
		deletedRaw sql.NullTime
	)
	if err := row.Scan(&node.ID, &node.OwnerUserID, &node.Type, &node.Name, &node.SizeBytes, &mimeRaw, &node.UpdatedAt, &deletedRaw); err != nil {
		return FilesNodeSummary{}, err
	}
	if mimeRaw.Valid {
		value := strings.TrimSpace(mimeRaw.String)
		node.MIMEType = &value
	}
	if deletedRaw.Valid {
		value := deletedRaw.Time
		node.DeletedAt = &value
	}
	return node, nil
}

func scanShareWithNode(row shareScanner, node *FilesNodeSummary) (ShareRecord, error) {
	var (
		item          ShareRecord
		passwordRaw   sql.NullString
		expiresRaw    sql.NullTime
		maxRaw        sql.NullInt32
		revokedRaw    sql.NullTime
		nodeMimeRaw   sql.NullString
		nodeDeleted   sql.NullTime
	)
	if err := row.Scan(
		&item.ID,
		&item.NodeID,
		&item.OwnerUserID,
		&item.TokenHash,
		&passwordRaw,
		&expiresRaw,
		&maxRaw,
		&item.DownloadCount,
		&item.AllowDownload,
		&item.AllowPreview,
		&item.CreatedAt,
		&item.UpdatedAt,
		&revokedRaw,
		&node.ID,
		&node.OwnerUserID,
		&node.Type,
		&node.Name,
		&node.SizeBytes,
		&nodeMimeRaw,
		&node.UpdatedAt,
		&nodeDeleted,
	); err != nil {
		return ShareRecord{}, err
	}
	if passwordRaw.Valid {
		value := passwordRaw.String
		item.PasswordHash = &value
	}
	if expiresRaw.Valid {
		value := expiresRaw.Time
		item.ExpiresAt = &value
	}
	if maxRaw.Valid {
		value := int(maxRaw.Int32)
		item.MaxDownloads = &value
	}
	if revokedRaw.Valid {
		value := revokedRaw.Time
		item.RevokedAt = &value
	}
	if nodeMimeRaw.Valid {
		value := strings.TrimSpace(nodeMimeRaw.String)
		node.MIMEType = &value
	}
	if nodeDeleted.Valid {
		value := nodeDeleted.Time
		node.DeletedAt = &value
	}
	return item, nil
}
