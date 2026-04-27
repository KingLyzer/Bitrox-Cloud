DROP INDEX IF EXISTS nodes_active_name_unique_idx;

CREATE UNIQUE INDEX IF NOT EXISTS nodes_active_name_unique_idx
    ON nodes (owner_user_id, parent_id, lower(name))
    WHERE deleted_at IS NULL;
