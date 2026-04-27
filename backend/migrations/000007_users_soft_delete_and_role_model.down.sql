DROP INDEX IF EXISTS users_deleted_at_idx;

ALTER TABLE users
    DROP COLUMN IF EXISTS deleted_at;

UPDATE users
SET role = 'member',
    updated_at = now()
WHERE role = 'user';

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('owner', 'admin', 'member', 'viewer'));
