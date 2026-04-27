DROP TRIGGER IF EXISTS nodes_parent_invariants_trigger ON nodes;
DROP FUNCTION IF EXISTS enforce_nodes_parent_invariants();

DROP INDEX IF EXISTS upload_sessions_owner_active_idx;

DROP INDEX IF EXISTS upload_sessions_owner_idempotency_pending_idx;

CREATE UNIQUE INDEX IF NOT EXISTS upload_sessions_owner_idempotency_pending_idx
    ON upload_sessions(owner_user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL
      AND status = 'pending';

ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_cleanup_attempts_check;

ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_cleanup_status_check;

ALTER TABLE upload_sessions
    DROP COLUMN IF EXISTS cleanup_updated_at,
    DROP COLUMN IF EXISTS cleanup_last_error,
    DROP COLUMN IF EXISTS cleanup_attempts,
    DROP COLUMN IF EXISTS cleanup_status;

ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_status_check;

UPDATE upload_sessions
SET status = 'aborted',
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE status = 'finalizing';

ALTER TABLE upload_sessions
    ADD CONSTRAINT upload_sessions_status_check
    CHECK (status IN ('pending', 'completed', 'aborted'));
