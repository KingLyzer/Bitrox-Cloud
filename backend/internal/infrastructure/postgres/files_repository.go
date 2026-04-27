package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud/backend/internal/domain/files"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FilesRepository struct {
	pool *pgxpool.Pool
}

const uploadSessionColumns = `
id, owner_user_id, target_type, target_node_id, parent_id, file_name, expected_size_bytes,
chunk_size_bytes, expected_chunks, idempotency_key, request_fingerprint, status, uploaded_bytes,
finalized_node_id, finalized_version_id, object_key, mime_type, content_hash, expires_at, completed_at,
cleanup_status, cleanup_attempts, cleanup_last_error, cleanup_updated_at, created_at, updated_at
`

func NewFilesRepository(pool *pgxpool.Pool) *FilesRepository {
	return &FilesRepository{pool: pool}
}

func (r *FilesRepository) CreateFolder(ctx context.Context, input files.CreateFolderInput) (files.Node, error) {
	const query = `
INSERT INTO nodes (owner_user_id, parent_id, type, name, created_at, updated_at)
VALUES ($1, $2, 'folder', $3, now(), now())
RETURNING id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
          current_version_id, current_version_no, deleted_at, created_at, updated_at
`
	row := r.pool.QueryRow(ctx, query, input.OwnerUserID, input.ParentID, input.Name)
	node, err := scanNode(row)
	if err != nil {
		if isUniqueViolation(err) {
			return files.Node{}, files.ErrNameConflict
		}
		return files.Node{}, fmt.Errorf("create folder: %w", err)
	}
	return node, nil
}

func (r *FilesRepository) GetNodeByID(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (files.Node, error) {
	const query = `
SELECT id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
       current_version_id, current_version_no, deleted_at, created_at, updated_at
FROM nodes
WHERE id = $1 AND owner_user_id = $2
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, nodeID, ownerUserID)
	node, err := scanNode(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.Node{}, files.ErrNodeNotFound
		}
		return files.Node{}, fmt.Errorf("get node by id: %w", err)
	}
	return node, nil
}

func (r *FilesRepository) GetActiveNodeByID(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (files.Node, error) {
	const query = `
SELECT id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
       current_version_id, current_version_no, deleted_at, created_at, updated_at
FROM nodes
WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, nodeID, ownerUserID)
	node, err := scanNode(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.Node{}, files.ErrNodeNotFound
		}
		return files.Node{}, fmt.Errorf("get active node by id: %w", err)
	}
	return node, nil
}

func (r *FilesRepository) ListActiveNodesByParent(ctx context.Context, ownerUserID uuid.UUID, parentID *uuid.UUID) ([]files.Node, error) {
	query := `
SELECT id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
       current_version_id, current_version_no, deleted_at, created_at, updated_at
FROM nodes
WHERE owner_user_id = $1
  AND deleted_at IS NULL
`

	args := []any{ownerUserID}
	if parentID == nil {
		query += " AND parent_id IS NULL"
	} else {
		query += " AND parent_id = $2"
		args = append(args, *parentID)
	}
	query += " ORDER BY type ASC, lower(name) ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list active nodes by parent: %w", err)
	}
	defer rows.Close()

	nodes := make([]files.Node, 0)
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node rows: %w", err)
	}
	return nodes, nil
}

func (r *FilesRepository) IsNodeInSubtree(ctx context.Context, ownerUserID, ancestorNodeID, candidateNodeID uuid.UUID) (bool, error) {
	const query = `
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id
    FROM nodes
    WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
  UNION ALL
    SELECT n.id, n.parent_id
    FROM nodes n
    JOIN ancestors a ON n.id = a.parent_id
    WHERE n.owner_user_id = $2 AND n.deleted_at IS NULL
)
SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = $3)
`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, candidateNodeID, ownerUserID, ancestorNodeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check subtree containment: %w", err)
	}
	return exists, nil
}

func (r *FilesRepository) UpdateNodeNameAndParent(ctx context.Context, input files.UpdateNodeInput) (files.Node, error) {
	const query = `
UPDATE nodes
SET name = $3,
    parent_id = $4,
    updated_at = now()
WHERE id = $1
  AND owner_user_id = $2
  AND deleted_at IS NULL
RETURNING id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
          current_version_id, current_version_no, deleted_at, created_at, updated_at
`
	row := r.pool.QueryRow(ctx, query, input.NodeID, input.OwnerUserID, input.Name, input.ParentID)
	node, err := scanNode(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.Node{}, files.ErrNodeNotFound
		}
		if isUniqueViolation(err) {
			return files.Node{}, files.ErrNameConflict
		}
		return files.Node{}, fmt.Errorf("update node name and parent: %w", err)
	}
	return node, nil
}

func (r *FilesRepository) SoftDeleteNodeTree(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID, now time.Time) error {
	const query = `
WITH RECURSIVE subtree AS (
    SELECT id
    FROM nodes
    WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
  UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN subtree s ON n.parent_id = s.id
    WHERE n.owner_user_id = $2 AND n.deleted_at IS NULL
)
UPDATE nodes
SET deleted_at = $3, updated_at = now()
WHERE id IN (SELECT id FROM subtree)
  AND deleted_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, nodeID, ownerUserID, now)
	if err != nil {
		return fmt.Errorf("soft delete node tree: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return files.ErrNodeNotFound
	}
	return nil
}

func (r *FilesRepository) GetUserUsedBytes(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
SELECT COALESCE(SUM(size_bytes), 0)
FROM nodes
WHERE owner_user_id = $1
  AND type = 'file'
  AND deleted_at IS NULL
`
	var used int64
	if err := r.pool.QueryRow(ctx, query, userID).Scan(&used); err != nil {
		return 0, fmt.Errorf("get user used bytes from nodes: %w", err)
	}
	return used, nil
}

func (r *FilesRepository) CountActiveNodesByOwner(ctx context.Context, ownerUserID uuid.UUID) (int, error) {
	const query = `
SELECT COUNT(*)::int
FROM nodes
WHERE owner_user_id = $1
  AND deleted_at IS NULL
`
	var count int
	if err := r.pool.QueryRow(ctx, query, ownerUserID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active nodes by owner: %w", err)
	}
	return count, nil
}

func (r *FilesRepository) CountActiveUploadSessions(ctx context.Context, ownerUserID uuid.UUID) (int, error) {
	const query = `
SELECT COUNT(*)::int
FROM upload_sessions
WHERE owner_user_id = $1
  AND status IN ('pending', 'finalizing')
`
	var count int
	if err := r.pool.QueryRow(ctx, query, ownerUserID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active upload sessions: %w", err)
	}
	return count, nil
}

func (r *FilesRepository) GetUserStagedBytes(ctx context.Context, ownerUserID uuid.UUID) (int64, error) {
	const query = `
SELECT COALESCE(SUM(uploaded_bytes), 0)::bigint
FROM upload_sessions
WHERE owner_user_id = $1
  AND status IN ('pending', 'finalizing')
`
	var staged int64
	if err := r.pool.QueryRow(ctx, query, ownerUserID).Scan(&staged); err != nil {
		return 0, fmt.Errorf("get user staged bytes: %w", err)
	}
	return staged, nil
}

func (r *FilesRepository) CreateUploadSession(ctx context.Context, input files.CreateUploadSessionInput) (files.UploadSession, error) {
	query := `
INSERT INTO upload_sessions (
    owner_user_id, target_type, target_node_id, parent_id, file_name, expected_size_bytes,
    chunk_size_bytes, expected_chunks, idempotency_key, request_fingerprint, expires_at,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    now(), now()
)
RETURNING ` + uploadSessionColumns
	row := r.pool.QueryRow(ctx, query,
		input.OwnerUserID,
		string(input.TargetType),
		input.TargetNodeID,
		input.ParentID,
		input.FileName,
		input.ExpectedSizeBytes,
		input.ChunkSizeBytes,
		input.ExpectedChunks,
		input.IdempotencyKey,
		input.RequestFingerprint,
		input.ExpiresAt,
	)

	session, err := scanUploadSession(row)
	if err != nil {
		if isUniqueViolation(err) && input.IdempotencyKey != nil {
			existing, getErr := r.GetPendingUploadSessionByIdempotencyKey(ctx, input.OwnerUserID, *input.IdempotencyKey)
			if getErr != nil {
				return files.UploadSession{}, fmt.Errorf("resolve idempotent create conflict: %w", getErr)
			}
			if input.RequestFingerprint != nil && existing.RequestFingerprint != nil && *existing.RequestFingerprint != *input.RequestFingerprint {
				return files.UploadSession{}, files.ErrIdempotencyConflict
			}
			return existing, nil
		}
		return files.UploadSession{}, fmt.Errorf("create upload session: %w", err)
	}
	return session, nil
}

func (r *FilesRepository) GetPendingUploadSessionByIdempotencyKey(ctx context.Context, ownerUserID uuid.UUID, idempotencyKey string) (files.UploadSession, error) {
	query := `
SELECT ` + uploadSessionColumns + `
FROM upload_sessions
WHERE owner_user_id = $1
  AND idempotency_key = $2
  AND status IN ('pending', 'finalizing')
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, ownerUserID, idempotencyKey)
	session, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrUploadSessionNotFound
		}
		return files.UploadSession{}, fmt.Errorf("get pending upload session by idempotency key: %w", err)
	}
	return session, nil
}

func (r *FilesRepository) GetUploadSessionByID(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID) (files.UploadSession, error) {
	query := `
SELECT ` + uploadSessionColumns + `
FROM upload_sessions
WHERE id = $1 AND owner_user_id = $2
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, uploadSessionID, ownerUserID)
	session, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrUploadSessionNotFound
		}
		return files.UploadSession{}, fmt.Errorf("get upload session by id: %w", err)
	}
	return session, nil
}

func (r *FilesRepository) ListExpiredActiveUploadSessions(ctx context.Context, now time.Time, limit int) ([]files.UploadSession, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
SELECT ` + uploadSessionColumns + `
FROM upload_sessions
WHERE status IN ('pending', 'finalizing')
  AND expires_at <= $1
ORDER BY expires_at ASC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired active upload sessions: %w", err)
	}
	defer rows.Close()

	out := make([]files.UploadSession, 0)
	for rows.Next() {
		session, err := scanUploadSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired upload session: %w", err)
		}
		out = append(out, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired upload sessions: %w", err)
	}
	return out, nil
}

func (r *FilesRepository) MarkUploadSessionCleanupInProgress(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time) (files.UploadSession, error) {
	query := `
UPDATE upload_sessions
SET status = 'aborted',
    completed_at = COALESCE(completed_at, $3),
    cleanup_status = 'in_progress',
    cleanup_attempts = cleanup_attempts + 1,
    cleanup_updated_at = now(),
    cleanup_last_error = NULL,
    updated_at = now()
WHERE id = $1
  AND owner_user_id = $2
  AND status IN ('pending', 'finalizing')
RETURNING ` + uploadSessionColumns + `
`
	row := r.pool.QueryRow(ctx, query, uploadSessionID, ownerUserID, now)
	session, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrUploadSessionNotFound
		}
		return files.UploadSession{}, fmt.Errorf("mark upload session cleanup in progress: %w", err)
	}
	return session, nil
}

func (r *FilesRepository) MarkUploadSessionCleanupResult(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time, cleanupErr error) error {
	status := "done"
	var errText any = nil
	if cleanupErr != nil {
		status = "failed"
		errText = cleanupErr.Error()
	}

	const query = `
UPDATE upload_sessions
SET cleanup_status = $3,
    cleanup_last_error = $4,
    cleanup_updated_at = $5,
    updated_at = now()
WHERE id = $1
  AND owner_user_id = $2
`
	if _, err := r.pool.Exec(ctx, query, uploadSessionID, ownerUserID, status, errText, now); err != nil {
		return fmt.Errorf("mark upload session cleanup result: %w", err)
	}
	return nil
}

func (r *FilesRepository) ListUploadChunks(ctx context.Context, uploadSessionID uuid.UUID) ([]files.UploadChunk, error) {
	const query = `
SELECT upload_session_id, chunk_index, size_bytes, content_hash, staging_key, created_at
FROM upload_chunks
WHERE upload_session_id = $1
ORDER BY chunk_index ASC
`
	rows, err := r.pool.Query(ctx, query, uploadSessionID)
	if err != nil {
		return nil, fmt.Errorf("list upload chunks: %w", err)
	}
	defer rows.Close()

	out := make([]files.UploadChunk, 0)
	for rows.Next() {
		var c files.UploadChunk
		if err := rows.Scan(&c.UploadSessionID, &c.ChunkIndex, &c.SizeBytes, &c.ContentHash, &c.StagingKey, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan upload chunk: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upload chunks: %w", err)
	}
	return out, nil
}

func (r *FilesRepository) UpsertUploadChunk(ctx context.Context, input files.UpsertUploadChunkInput) (files.UploadSession, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return files.UploadSession{}, fmt.Errorf("begin upload chunk transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	session, err := getUploadSessionForUpdate(ctx, tx, input.OwnerUserID, input.UploadSessionID)
	if err != nil {
		return files.UploadSession{}, err
	}
	if session.Status == files.UploadSessionStatusCompleted {
		return files.UploadSession{}, files.ErrInvalidOperation
	}
	if session.Status == files.UploadSessionStatusFinalizing {
		return files.UploadSession{}, files.ErrFinalizeInProgress
	}
	if session.Status == files.UploadSessionStatusAborted {
		return files.UploadSession{}, files.ErrUploadAborted
	}
	if !input.Now.Before(session.ExpiresAt) {
		return files.UploadSession{}, files.ErrUploadExpired
	}
	if input.ChunkIndex < 0 || input.ChunkIndex >= session.ExpectedChunks {
		return files.UploadSession{}, files.ErrInvalidOperation
	}

	var (
		existingSize int
		existingHash string
		exists       bool
	)

	const existingChunkQuery = `
SELECT size_bytes, content_hash
FROM upload_chunks
WHERE upload_session_id = $1
  AND chunk_index = $2
FOR UPDATE
`
	if err := tx.QueryRow(ctx, existingChunkQuery, input.UploadSessionID, input.ChunkIndex).Scan(&existingSize, &existingHash); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, fmt.Errorf("load existing upload chunk: %w", err)
		}
	} else {
		exists = true
	}

	chunkDelta := int64(input.SizeBytes)
	if exists {
		if existingSize != input.SizeBytes || !strings.EqualFold(strings.TrimSpace(existingHash), strings.TrimSpace(input.ContentHash)) {
			return files.UploadSession{}, files.ErrChunkConflict
		}
		chunkDelta = 0
	}

	if session.UploadedBytes+chunkDelta > session.ExpectedSizeBytes {
		return files.UploadSession{}, files.ErrInvalidOperation
	}

	if input.MaxUserStagedBytes > 0 && chunkDelta > 0 {
		if err := lockUserForStagedQuota(ctx, tx, input.OwnerUserID); err != nil {
			return files.UploadSession{}, err
		}
		stagedBytes, err := getUserStagedBytesForUpdate(ctx, tx, input.OwnerUserID)
		if err != nil {
			return files.UploadSession{}, err
		}
		if stagedBytes+chunkDelta > input.MaxUserStagedBytes {
			return files.UploadSession{}, files.ErrStagedQuotaExceeded
		}
	}

	if !exists {
		const insertChunk = `
INSERT INTO upload_chunks (upload_session_id, chunk_index, size_bytes, content_hash, staging_key, created_at)
VALUES ($1, $2, $3, $4, $5, now())
`
		if _, err := tx.Exec(ctx, insertChunk, input.UploadSessionID, input.ChunkIndex, input.SizeBytes, input.ContentHash, input.StagingKey); err != nil {
			return files.UploadSession{}, fmt.Errorf("insert upload chunk: %w", err)
		}
	}

	const refreshProgress = `
UPDATE upload_sessions us
SET uploaded_bytes = us.uploaded_bytes + $2,
    updated_at = now()
WHERE us.id = $1
RETURNING ` + uploadSessionColumns
	row := tx.QueryRow(ctx, refreshProgress, input.UploadSessionID, chunkDelta)
	updatedSession, err := scanUploadSession(row)
	if err != nil {
		return files.UploadSession{}, fmt.Errorf("refresh upload progress: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return files.UploadSession{}, fmt.Errorf("commit upload chunk transaction: %w", err)
	}
	return updatedSession, nil
}

func (r *FilesRepository) AbortUploadSession(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID, now time.Time) (files.UploadSession, error) {
	query := `
UPDATE upload_sessions
SET status = 'aborted', updated_at = now(), completed_at = CASE WHEN completed_at IS NULL THEN $3 ELSE completed_at END
WHERE id = $1
  AND owner_user_id = $2
  AND status IN ('pending', 'finalizing')
RETURNING ` + uploadSessionColumns
	row := r.pool.QueryRow(ctx, query, uploadSessionID, ownerUserID, now)
	session, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrUploadSessionNotFound
		}
		return files.UploadSession{}, fmt.Errorf("abort upload session: %w", err)
	}
	return session, nil
}

func (r *FilesRepository) BeginFinalizeUploadSession(ctx context.Context, input files.BeginFinalizeUploadInput) (files.UploadSession, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return files.UploadSession{}, fmt.Errorf("begin finalize session transition transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	session, err := getUploadSessionForUpdate(ctx, tx, input.OwnerUserID, input.UploadSessionID)
	if err != nil {
		return files.UploadSession{}, err
	}

	switch session.Status {
	case files.UploadSessionStatusCompleted:
		if err := tx.Commit(ctx); err != nil {
			return files.UploadSession{}, fmt.Errorf("commit finalize already-completed transition: %w", err)
		}
		return session, nil
	case files.UploadSessionStatusAborted:
		return files.UploadSession{}, files.ErrUploadAborted
	case files.UploadSessionStatusFinalizing:
		if session.ObjectKey != nil && strings.TrimSpace(*session.ObjectKey) == strings.TrimSpace(input.ObjectKey) {
			if err := tx.Commit(ctx); err != nil {
				return files.UploadSession{}, fmt.Errorf("commit finalize same-key transition: %w", err)
			}
			return session, nil
		}
		return files.UploadSession{}, files.ErrFinalizeInProgress
	case files.UploadSessionStatusPending:
		if !input.Now.Before(session.ExpiresAt) {
			return files.UploadSession{}, files.ErrUploadExpired
		}
		if err := validateUploadChunksCompleteness(ctx, tx, session.ID, session.ExpectedChunks, session.ExpectedSizeBytes); err != nil {
			return files.UploadSession{}, err
		}
	default:
		return files.UploadSession{}, files.ErrInvalidOperation
	}

	const query = `
UPDATE upload_sessions
SET status = 'finalizing',
    object_key = $3,
    updated_at = now()
WHERE id = $1
  AND owner_user_id = $2
  AND status = 'pending'
RETURNING ` + uploadSessionColumns + `
`
	row := tx.QueryRow(ctx, query, input.UploadSessionID, input.OwnerUserID, input.ObjectKey)
	updatedSession, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrFinalizeInProgress
		}
		return files.UploadSession{}, fmt.Errorf("mark upload session finalizing: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return files.UploadSession{}, fmt.Errorf("commit finalize session transition: %w", err)
	}
	return updatedSession, nil
}

func (r *FilesRepository) GetCompletedFinalizeResult(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID) (files.FinalizeUploadResult, error) {
	session, err := r.GetUploadSessionByID(ctx, ownerUserID, uploadSessionID)
	if err != nil {
		return files.FinalizeUploadResult{}, err
	}
	if session.Status != files.UploadSessionStatusCompleted {
		return files.FinalizeUploadResult{}, files.ErrInvalidOperation
	}
	if session.FinalizedNodeID == nil || session.FinalizedVersionID == nil {
		return files.FinalizeUploadResult{}, files.ErrInvalidOperation
	}

	node, err := r.GetNodeByID(ctx, ownerUserID, *session.FinalizedNodeID)
	if err != nil {
		return files.FinalizeUploadResult{}, err
	}

	const versionQuery = `
SELECT id, node_id, version_no, storage_key, size_bytes, mime_type, content_hash, created_by_user_id, created_at
FROM file_versions
WHERE id = $1
LIMIT 1
`
	var version files.FileVersion
	if err := r.pool.QueryRow(ctx, versionQuery, *session.FinalizedVersionID).Scan(
		&version.ID,
		&version.NodeID,
		&version.VersionNo,
		&version.StorageKey,
		&version.SizeBytes,
		&version.MIMEType,
		&version.ContentHash,
		&version.CreatedByUserID,
		&version.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.FinalizeUploadResult{}, files.ErrInvalidOperation
		}
		return files.FinalizeUploadResult{}, fmt.Errorf("load finalized file version: %w", err)
	}

	return files.FinalizeUploadResult{
		Session:         session,
		Node:            node,
		Version:         version,
		WasAlreadyFinal: true,
	}, nil
}

func (r *FilesRepository) FinalizeUploadSession(ctx context.Context, input files.FinalizeUploadInput) (files.FinalizeUploadResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return files.FinalizeUploadResult{}, fmt.Errorf("begin finalize upload transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	session, err := getUploadSessionForUpdate(ctx, tx, input.OwnerUserID, input.UploadSessionID)
	if err != nil {
		return files.FinalizeUploadResult{}, err
	}

	if session.Status == files.UploadSessionStatusCompleted {
		if session.FinalizedNodeID == nil || session.FinalizedVersionID == nil {
			return files.FinalizeUploadResult{}, fmt.Errorf("completed upload session missing finalized references")
		}
		node, err := getNodeByIDForUpdate(ctx, tx, input.OwnerUserID, *session.FinalizedNodeID)
		if err != nil {
			return files.FinalizeUploadResult{}, err
		}
		version, err := getFileVersionByID(ctx, tx, *session.FinalizedVersionID)
		if err != nil {
			return files.FinalizeUploadResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return files.FinalizeUploadResult{}, &files.CommitUnknownError{
				Operation: "finalize_upload_session_read_completed",
				Cause:     err,
			}
		}
		return files.FinalizeUploadResult{
			Session:         session,
			Node:            node,
			Version:         version,
			WasAlreadyFinal: true,
		}, nil
	}
	if session.Status == files.UploadSessionStatusAborted {
		return files.FinalizeUploadResult{}, files.ErrUploadAborted
	}
	if session.Status != files.UploadSessionStatusFinalizing {
		return files.FinalizeUploadResult{}, files.ErrInvalidOperation
	}
	if session.ObjectKey == nil || strings.TrimSpace(*session.ObjectKey) == "" {
		return files.FinalizeUploadResult{}, files.ErrInvalidOperation
	}
	if strings.TrimSpace(*session.ObjectKey) != strings.TrimSpace(input.ObjectKey) {
		return files.FinalizeUploadResult{}, files.ErrFinalizeInProgress
	}

	if err := validateUploadChunksCompleteness(ctx, tx, session.ID, session.ExpectedChunks, session.ExpectedSizeBytes); err != nil {
		return files.FinalizeUploadResult{}, err
	}

	node, version, err := r.finalizeNodeAndVersion(ctx, tx, session, input)
	if err != nil {
		return files.FinalizeUploadResult{}, err
	}

	const completeSession = `
UPDATE upload_sessions
SET status = 'completed',
    finalized_node_id = $2,
    finalized_version_id = $3,
    object_key = $4,
    mime_type = $5,
    content_hash = $6,
    completed_at = $7,
    cleanup_status = 'none',
    cleanup_last_error = NULL,
    cleanup_updated_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING ` + uploadSessionColumns + `
`
	row := tx.QueryRow(ctx, completeSession, session.ID, node.ID, version.ID, input.ObjectKey, input.MIMEType, input.ContentHash, input.Now)
	updatedSession, err := scanUploadSession(row)
	if err != nil {
		return files.FinalizeUploadResult{}, fmt.Errorf("complete upload session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return files.FinalizeUploadResult{}, &files.CommitUnknownError{
			Operation: "finalize_upload_session_commit",
			Cause:     err,
		}
	}

	return files.FinalizeUploadResult{
		Session: updatedSession,
		Node:    node,
		Version: version,
	}, nil
}

func (r *FilesRepository) finalizeNodeAndVersion(ctx context.Context, tx pgx.Tx, session files.UploadSession, input files.FinalizeUploadInput) (files.Node, files.FileVersion, error) {
	switch session.TargetType {
	case files.UploadTargetTypeNewFile:
		return finalizeNewFileNode(ctx, tx, session, input)
	case files.UploadTargetTypeNewVersion:
		return finalizeNewVersionNode(ctx, tx, session, input)
	default:
		return files.Node{}, files.FileVersion{}, files.ErrInvalidOperation
	}
}

func finalizeNewFileNode(ctx context.Context, tx pgx.Tx, session files.UploadSession, input files.FinalizeUploadInput) (files.Node, files.FileVersion, error) {
	if session.ParentID != nil {
		parent, err := getNodeByIDForUpdate(ctx, tx, session.OwnerUserID, *session.ParentID)
		if err != nil {
			return files.Node{}, files.FileVersion{}, err
		}
		if parent.Type != files.NodeTypeFolder || parent.DeletedAt != nil {
			return files.Node{}, files.FileVersion{}, files.ErrInvalidOperation
		}
	}

	if err := enforceQuotaForTransition(ctx, tx, session.OwnerUserID, input.SizeBytes); err != nil {
		return files.Node{}, files.FileVersion{}, err
	}

	const createNode = `
INSERT INTO nodes (owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key, current_version_no, created_at, updated_at)
VALUES ($1, $2, 'file', $3, $4, $5, $6, $7, 1, now(), now())
RETURNING id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
          current_version_id, current_version_no, deleted_at, created_at, updated_at
`
	row := tx.QueryRow(ctx, createNode, session.OwnerUserID, session.ParentID, session.FileName, input.SizeBytes, input.MIMEType, input.ContentHash, input.ObjectKey)
	node, err := scanNode(row)
	if err != nil {
		if isUniqueViolation(err) {
			return files.Node{}, files.FileVersion{}, files.ErrNameConflict
		}
		return files.Node{}, files.FileVersion{}, fmt.Errorf("create file node: %w", err)
	}

	version, err := insertFileVersion(ctx, tx, node.ID, 1, input.ObjectKey, input.SizeBytes, input.MIMEType, input.ContentHash, session.OwnerUserID)
	if err != nil {
		return files.Node{}, files.FileVersion{}, err
	}

	const setCurrentVersion = `
UPDATE nodes
SET current_version_id = $2, updated_at = now()
WHERE id = $1
RETURNING id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
          current_version_id, current_version_no, deleted_at, created_at, updated_at
`
	row = tx.QueryRow(ctx, setCurrentVersion, node.ID, version.ID)
	node, err = scanNode(row)
	if err != nil {
		return files.Node{}, files.FileVersion{}, fmt.Errorf("set current version on new file node: %w", err)
	}
	return node, version, nil
}

func finalizeNewVersionNode(ctx context.Context, tx pgx.Tx, session files.UploadSession, input files.FinalizeUploadInput) (files.Node, files.FileVersion, error) {
	if session.TargetNodeID == nil {
		return files.Node{}, files.FileVersion{}, files.ErrInvalidOperation
	}

	node, err := getNodeByIDForUpdate(ctx, tx, session.OwnerUserID, *session.TargetNodeID)
	if err != nil {
		return files.Node{}, files.FileVersion{}, err
	}
	if node.Type != files.NodeTypeFile || node.DeletedAt != nil {
		return files.Node{}, files.FileVersion{}, files.ErrInvalidOperation
	}

	delta := input.SizeBytes - node.SizeBytes
	if err := enforceQuotaForTransition(ctx, tx, session.OwnerUserID, delta); err != nil {
		return files.Node{}, files.FileVersion{}, err
	}

	nextVersionNo := 1
	if node.CurrentVersionNo != nil {
		nextVersionNo = *node.CurrentVersionNo + 1
	}
	version, err := insertFileVersion(ctx, tx, node.ID, nextVersionNo, input.ObjectKey, input.SizeBytes, input.MIMEType, input.ContentHash, session.OwnerUserID)
	if err != nil {
		return files.Node{}, files.FileVersion{}, err
	}

	const updateNode = `
UPDATE nodes
SET size_bytes = $2,
    mime_type = $3,
    content_hash = $4,
    storage_key = $5,
    current_version_id = $6,
    current_version_no = $7,
    updated_at = now()
WHERE id = $1
RETURNING id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
          current_version_id, current_version_no, deleted_at, created_at, updated_at
`
	row := tx.QueryRow(ctx, updateNode, node.ID, input.SizeBytes, input.MIMEType, input.ContentHash, input.ObjectKey, version.ID, nextVersionNo)
	node, err = scanNode(row)
	if err != nil {
		return files.Node{}, files.FileVersion{}, fmt.Errorf("update file node with new version: %w", err)
	}
	return node, version, nil
}

func enforceQuotaForTransition(ctx context.Context, tx pgx.Tx, ownerUserID uuid.UUID, sizeDelta int64) error {
	additional := sizeDelta
	if additional < 0 {
		additional = 0
	}

	const query = `
SELECT
    COALESCE(u.quota_bytes, qp.quota_bytes) AS effective_quota,
    COALESCE((
        SELECT SUM(n.size_bytes)
        FROM nodes n
        WHERE n.owner_user_id = u.id
          AND n.type = 'file'
          AND n.deleted_at IS NULL
    ), 0) AS used_bytes
FROM users u
JOIN quota_policies qp ON qp.key = 'default_user_quota'
WHERE u.id = $1
FOR UPDATE
`
	var limit int64
	var used int64
	if err := tx.QueryRow(ctx, query, ownerUserID).Scan(&limit, &used); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.ErrInvalidOperation
		}
		return fmt.Errorf("load quota for finalize transition: %w", err)
	}
	if used+additional > limit {
		return files.ErrQuotaExceeded
	}
	return nil
}

func validateUploadChunksCompleteness(ctx context.Context, tx pgx.Tx, uploadSessionID uuid.UUID, expectedChunks int, expectedSizeBytes int64) error {
	const query = `
SELECT
    COALESCE(COUNT(*), 0)::int AS chunk_count,
    COALESCE(SUM(size_bytes), 0)::bigint AS total_size,
    COALESCE(MIN(chunk_index), 0)::int AS min_index,
    COALESCE(MAX(chunk_index), -1)::int AS max_index
FROM upload_chunks
WHERE upload_session_id = $1
`
	var count int
	var total int64
	var minIdx int
	var maxIdx int
	if err := tx.QueryRow(ctx, query, uploadSessionID).Scan(&count, &total, &minIdx, &maxIdx); err != nil {
		return fmt.Errorf("validate upload chunk completeness aggregate: %w", err)
	}
	if count != expectedChunks {
		return files.ErrUploadIncomplete
	}
	if total != expectedSizeBytes {
		return files.ErrUploadIncomplete
	}
	if minIdx != 0 || maxIdx != expectedChunks-1 {
		return files.ErrUploadIncomplete
	}
	return nil
}

func lockUserForStagedQuota(ctx context.Context, tx pgx.Tx, ownerUserID uuid.UUID) error {
	const query = `
SELECT id
FROM users
WHERE id = $1
FOR UPDATE
`
	var userID uuid.UUID
	if err := tx.QueryRow(ctx, query, ownerUserID).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.ErrInvalidOperation
		}
		return fmt.Errorf("lock user row for staged quota: %w", err)
	}
	return nil
}

func getUserStagedBytesForUpdate(ctx context.Context, tx pgx.Tx, ownerUserID uuid.UUID) (int64, error) {
	const query = `
SELECT COALESCE(SUM(uploaded_bytes), 0)::bigint
FROM upload_sessions
WHERE owner_user_id = $1
  AND status IN ('pending', 'finalizing')
`
	var staged int64
	if err := tx.QueryRow(ctx, query, ownerUserID).Scan(&staged); err != nil {
		return 0, fmt.Errorf("get user staged bytes for update: %w", err)
	}
	return staged, nil
}

func insertFileVersion(
	ctx context.Context,
	tx pgx.Tx,
	nodeID uuid.UUID,
	versionNo int,
	storageKey string,
	sizeBytes int64,
	mimeType string,
	contentHash string,
	createdByUserID uuid.UUID,
) (files.FileVersion, error) {
	const query = `
INSERT INTO file_versions (node_id, version_no, storage_key, size_bytes, mime_type, content_hash, created_by_user_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, node_id, version_no, storage_key, size_bytes, mime_type, content_hash, created_by_user_id, created_at
`
	row := tx.QueryRow(ctx, query, nodeID, versionNo, storageKey, sizeBytes, mimeType, contentHash, createdByUserID)
	var version files.FileVersion
	if err := row.Scan(
		&version.ID,
		&version.NodeID,
		&version.VersionNo,
		&version.StorageKey,
		&version.SizeBytes,
		&version.MIMEType,
		&version.ContentHash,
		&version.CreatedByUserID,
		&version.CreatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return files.FileVersion{}, files.ErrInvalidOperation
		}
		return files.FileVersion{}, fmt.Errorf("insert file version: %w", err)
	}
	return version, nil
}

func getNodeByIDForUpdate(ctx context.Context, tx pgx.Tx, ownerUserID, nodeID uuid.UUID) (files.Node, error) {
	const query = `
SELECT id, owner_user_id, parent_id, type, name, size_bytes, mime_type, content_hash, storage_key,
       current_version_id, current_version_no, deleted_at, created_at, updated_at
FROM nodes
WHERE id = $1
  AND owner_user_id = $2
FOR UPDATE
`
	row := tx.QueryRow(ctx, query, nodeID, ownerUserID)
	node, err := scanNode(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.Node{}, files.ErrNodeNotFound
		}
		return files.Node{}, fmt.Errorf("get node for update: %w", err)
	}
	return node, nil
}

func getFileVersionByID(ctx context.Context, tx pgx.Tx, versionID uuid.UUID) (files.FileVersion, error) {
	const query = `
SELECT id, node_id, version_no, storage_key, size_bytes, mime_type, content_hash, created_by_user_id, created_at
FROM file_versions
WHERE id = $1
LIMIT 1
`
	row := tx.QueryRow(ctx, query, versionID)
	var version files.FileVersion
	if err := row.Scan(
		&version.ID,
		&version.NodeID,
		&version.VersionNo,
		&version.StorageKey,
		&version.SizeBytes,
		&version.MIMEType,
		&version.ContentHash,
		&version.CreatedByUserID,
		&version.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.FileVersion{}, files.ErrInvalidOperation
		}
		return files.FileVersion{}, fmt.Errorf("get file version by id: %w", err)
	}
	return version, nil
}

func getUploadSessionForUpdate(ctx context.Context, tx pgx.Tx, ownerUserID, uploadSessionID uuid.UUID) (files.UploadSession, error) {
	query := `
SELECT ` + uploadSessionColumns + `
FROM upload_sessions
WHERE id = $1 AND owner_user_id = $2
FOR UPDATE
`
	row := tx.QueryRow(ctx, query, uploadSessionID, ownerUserID)
	session, err := scanUploadSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.UploadSession{}, files.ErrUploadSessionNotFound
		}
		return files.UploadSession{}, fmt.Errorf("get upload session for update: %w", err)
	}
	return session, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanNode(row scannable) (files.Node, error) {
	var node files.Node
	var (
		parentID         sql.NullString
		nodeType         string
		mimeType         sql.NullString
		contentHash      sql.NullString
		storageKey       sql.NullString
		currentVersionID sql.NullString
		currentVersionNo sql.NullInt32
		deletedAt        sql.NullTime
	)

	if err := row.Scan(
		&node.ID,
		&node.OwnerUserID,
		&parentID,
		&nodeType,
		&node.Name,
		&node.SizeBytes,
		&mimeType,
		&contentHash,
		&storageKey,
		&currentVersionID,
		&currentVersionNo,
		&deletedAt,
		&node.CreatedAt,
		&node.UpdatedAt,
	); err != nil {
		return files.Node{}, err
	}

	node.Type = files.NodeType(strings.TrimSpace(nodeType))
	if parentID.Valid {
		v, err := uuid.Parse(parentID.String)
		if err != nil {
			return files.Node{}, fmt.Errorf("parse node parent id: %w", err)
		}
		node.ParentID = &v
	}
	if mimeType.Valid {
		v := mimeType.String
		node.MIMEType = &v
	}
	if contentHash.Valid {
		v := contentHash.String
		node.ContentHash = &v
	}
	if storageKey.Valid {
		v := storageKey.String
		node.StorageKey = &v
	}
	if currentVersionID.Valid {
		v, err := uuid.Parse(currentVersionID.String)
		if err != nil {
			return files.Node{}, fmt.Errorf("parse node current version id: %w", err)
		}
		node.CurrentVersionID = &v
	}
	if currentVersionNo.Valid {
		v := int(currentVersionNo.Int32)
		node.CurrentVersionNo = &v
	}
	if deletedAt.Valid {
		v := deletedAt.Time
		node.DeletedAt = &v
	}
	return node, nil
}

func scanUploadSession(row scannable) (files.UploadSession, error) {
	var s files.UploadSession
	var (
		targetType         string
		targetNodeID       sql.NullString
		parentID           sql.NullString
		idempotencyKey     sql.NullString
		requestFingerprint sql.NullString
		status             string
		finalizedNodeID    sql.NullString
		finalizedVersionID sql.NullString
		objectKey          sql.NullString
		mimeType           sql.NullString
		contentHash        sql.NullString
		completedAt        sql.NullTime
		cleanupStatus      string
		cleanupAttempts    int
		cleanupLastError   sql.NullString
		cleanupUpdatedAt   sql.NullTime
	)

	if err := row.Scan(
		&s.ID,
		&s.OwnerUserID,
		&targetType,
		&targetNodeID,
		&parentID,
		&s.FileName,
		&s.ExpectedSizeBytes,
		&s.ChunkSizeBytes,
		&s.ExpectedChunks,
		&idempotencyKey,
		&requestFingerprint,
		&status,
		&s.UploadedBytes,
		&finalizedNodeID,
		&finalizedVersionID,
		&objectKey,
		&mimeType,
		&contentHash,
		&s.ExpiresAt,
		&completedAt,
		&cleanupStatus,
		&cleanupAttempts,
		&cleanupLastError,
		&cleanupUpdatedAt,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return files.UploadSession{}, err
	}

	s.TargetType = files.UploadTargetType(targetType)
	s.Status = files.UploadSessionStatus(status)
	if targetNodeID.Valid {
		v, err := uuid.Parse(targetNodeID.String)
		if err != nil {
			return files.UploadSession{}, fmt.Errorf("parse upload session target node id: %w", err)
		}
		s.TargetNodeID = &v
	}
	if parentID.Valid {
		v, err := uuid.Parse(parentID.String)
		if err != nil {
			return files.UploadSession{}, fmt.Errorf("parse upload session parent id: %w", err)
		}
		s.ParentID = &v
	}
	if idempotencyKey.Valid {
		v := idempotencyKey.String
		s.IdempotencyKey = &v
	}
	if requestFingerprint.Valid {
		v := requestFingerprint.String
		s.RequestFingerprint = &v
	}
	if finalizedNodeID.Valid {
		v, err := uuid.Parse(finalizedNodeID.String)
		if err != nil {
			return files.UploadSession{}, fmt.Errorf("parse upload session finalized node id: %w", err)
		}
		s.FinalizedNodeID = &v
	}
	if finalizedVersionID.Valid {
		v, err := uuid.Parse(finalizedVersionID.String)
		if err != nil {
			return files.UploadSession{}, fmt.Errorf("parse upload session finalized version id: %w", err)
		}
		s.FinalizedVersionID = &v
	}
	if objectKey.Valid {
		v := objectKey.String
		s.ObjectKey = &v
	}
	if mimeType.Valid {
		v := mimeType.String
		s.MIMEType = &v
	}
	if contentHash.Valid {
		v := contentHash.String
		s.ContentHash = &v
	}
	if completedAt.Valid {
		v := completedAt.Time
		s.CompletedAt = &v
	}
	s.CleanupStatus = cleanupStatus
	s.CleanupAttempts = cleanupAttempts
	if cleanupLastError.Valid {
		v := cleanupLastError.String
		s.CleanupLastError = &v
	}
	if cleanupUpdatedAt.Valid {
		v := cleanupUpdatedAt.Time
		s.CleanupUpdatedAt = &v
	}
	return s, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
