ALTER TABLE nodes DROP COLUMN IF EXISTS current_version_id;

DROP TABLE IF EXISTS upload_chunks;
DROP INDEX IF EXISTS upload_sessions_owner_idempotency_pending_idx;
DROP INDEX IF EXISTS upload_sessions_expires_at_idx;
DROP INDEX IF EXISTS upload_sessions_owner_status_idx;
DROP TABLE IF EXISTS upload_sessions;

DROP TABLE IF EXISTS file_versions;

DROP INDEX IF EXISTS nodes_active_name_unique_idx;
DROP INDEX IF EXISTS nodes_deleted_at_idx;
DROP INDEX IF EXISTS nodes_parent_idx;
DROP INDEX IF EXISTS nodes_owner_parent_idx;
DROP TABLE IF EXISTS nodes;
