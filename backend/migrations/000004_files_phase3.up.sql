CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID NULL REFERENCES nodes(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('file', 'folder')),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    size_bytes BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    mime_type TEXT NULL,
    content_hash TEXT NULL,
    storage_key TEXT NULL,
    current_version_no INTEGER NULL CHECK (current_version_no IS NULL OR current_version_no > 0),
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS nodes_owner_parent_idx ON nodes (owner_user_id, parent_id);
CREATE INDEX IF NOT EXISTS nodes_parent_idx ON nodes (parent_id);
CREATE INDEX IF NOT EXISTS nodes_deleted_at_idx ON nodes (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS nodes_active_name_unique_idx
    ON nodes (owner_user_id, parent_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS file_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    version_no INTEGER NOT NULL CHECK (version_no > 0),
    storage_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, version_no)
);

CREATE TABLE IF NOT EXISTS upload_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('new_file', 'new_version')),
    target_node_id UUID NULL REFERENCES nodes(id) ON DELETE SET NULL,
    parent_id UUID NULL REFERENCES nodes(id) ON DELETE SET NULL,
    file_name TEXT NOT NULL CHECK (length(trim(file_name)) > 0),
    expected_size_bytes BIGINT NOT NULL CHECK (expected_size_bytes > 0),
    chunk_size_bytes INTEGER NOT NULL CHECK (chunk_size_bytes > 0),
    expected_chunks INTEGER NOT NULL CHECK (expected_chunks > 0),
    idempotency_key TEXT NULL,
    request_fingerprint TEXT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'aborted')),
    uploaded_bytes BIGINT NOT NULL DEFAULT 0 CHECK (uploaded_bytes >= 0),
    finalized_node_id UUID NULL REFERENCES nodes(id) ON DELETE SET NULL,
    finalized_version_id UUID NULL REFERENCES file_versions(id) ON DELETE SET NULL,
    object_key TEXT NULL,
    mime_type TEXT NULL,
    content_hash TEXT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS upload_sessions_owner_status_idx ON upload_sessions(owner_user_id, status);
CREATE INDEX IF NOT EXISTS upload_sessions_expires_at_idx ON upload_sessions(expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS upload_sessions_owner_idempotency_pending_idx
    ON upload_sessions(owner_user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND status = 'pending';

CREATE TABLE IF NOT EXISTS upload_chunks (
    upload_session_id UUID NOT NULL REFERENCES upload_sessions(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL CHECK (chunk_index >= 0),
    size_bytes INTEGER NOT NULL CHECK (size_bytes > 0),
    content_hash TEXT NOT NULL,
    staging_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (upload_session_id, chunk_index)
);

ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS current_version_id UUID NULL REFERENCES file_versions(id) ON DELETE SET NULL;
