UPDATE users
SET role = 'user',
    updated_at = now()
WHERE role IN ('member', 'viewer');

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('owner', 'admin', 'user'));

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS users_deleted_at_idx ON users (deleted_at);
