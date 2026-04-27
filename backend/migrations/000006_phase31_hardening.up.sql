ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_status_check;

ALTER TABLE upload_sessions
    ADD CONSTRAINT upload_sessions_status_check
    CHECK (status IN ('pending', 'finalizing', 'completed', 'aborted'));

ALTER TABLE upload_sessions
    ADD COLUMN IF NOT EXISTS cleanup_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS cleanup_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cleanup_last_error TEXT NULL,
    ADD COLUMN IF NOT EXISTS cleanup_updated_at TIMESTAMPTZ NULL;

ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_cleanup_status_check;

ALTER TABLE upload_sessions
    ADD CONSTRAINT upload_sessions_cleanup_status_check
    CHECK (cleanup_status IN ('none', 'in_progress', 'done', 'failed'));

ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS upload_sessions_cleanup_attempts_check;

ALTER TABLE upload_sessions
    ADD CONSTRAINT upload_sessions_cleanup_attempts_check
    CHECK (cleanup_attempts >= 0);

DROP INDEX IF EXISTS upload_sessions_owner_idempotency_pending_idx;

CREATE UNIQUE INDEX IF NOT EXISTS upload_sessions_owner_idempotency_pending_idx
    ON upload_sessions(owner_user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL
      AND status IN ('pending', 'finalizing');

CREATE INDEX IF NOT EXISTS upload_sessions_owner_active_idx
    ON upload_sessions(owner_user_id, status)
    WHERE status IN ('pending', 'finalizing');

CREATE OR REPLACE FUNCTION enforce_nodes_parent_invariants()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    parent_owner UUID;
    parent_type TEXT;
    parent_deleted_at TIMESTAMPTZ;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF NEW.parent_id = NEW.id THEN
        RAISE EXCEPTION 'node parent cannot reference itself';
    END IF;

    SELECT owner_user_id, type, deleted_at
    INTO parent_owner, parent_type, parent_deleted_at
    FROM nodes
    WHERE id = NEW.parent_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'parent node does not exist';
    END IF;

    IF parent_type <> 'folder' THEN
        RAISE EXCEPTION 'parent node must be a folder';
    END IF;

    IF parent_owner <> NEW.owner_user_id THEN
        RAISE EXCEPTION 'parent node must belong to the same owner';
    END IF;

    IF parent_deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'parent node is deleted';
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS nodes_parent_invariants_trigger ON nodes;

CREATE TRIGGER nodes_parent_invariants_trigger
    BEFORE INSERT OR UPDATE OF parent_id, owner_user_id
    ON nodes
    FOR EACH ROW
    EXECUTE FUNCTION enforce_nodes_parent_invariants();
