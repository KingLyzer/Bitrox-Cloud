DROP INDEX IF EXISTS nodes_active_name_unique_idx;

CREATE UNIQUE INDEX IF NOT EXISTS nodes_active_name_unique_idx
    ON nodes (
        owner_user_id,
        COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid),
        lower(name)
    )
    WHERE deleted_at IS NULL;
