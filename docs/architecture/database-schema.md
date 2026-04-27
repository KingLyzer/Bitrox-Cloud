# Database Schema (PostgreSQL)

## Conventions
- All tables use `uuid` primary keys unless noted.
- `created_at`, `updated_at` as `timestamptz`.
- Soft-delete where recovery/history is needed.
- Row-level authorization enforced in app layer (RLS optional future enhancement).

## Identity
- `users`: account profile, status, quota overrides, role.
- `credentials`: password hash metadata (`argon2id` params).
- `sessions`: refresh-token family, rotation, device metadata, revocation.
- `mfa_totp`: encrypted TOTP secret, enabled flag, recovery codes.
- `roles`: system roles (`owner`, `admin`, `member`, `viewer`).
- `role_bindings`: user->role scopes.

## Filesystem Metadata
- `nodes`: unified table for files/folders.
  - `type` (`file`|`folder`)
  - `parent_id` self-reference
  - `owner_user_id`, `storage_key`, `size_bytes`, `mime_type`
  - `content_hash`, `version_no`, `deleted_at`
- `file_versions`: immutable version records with storage pointers.
- `node_tags`, `tags`: tagging.
- `trash_entries`: trash metadata and purge scheduling.
- `upload_sessions`: chunked upload state, expiry, completed marker.

## Sharing and Access
- `share_links`: token hash, password hash, expiry, permissions.
- `node_acl`: explicit grants (`read`, `write`, `share`, `manage`) to users/roles.
- `group_memberships` (future small-team feature).

## Sync
- `sync_clients`: registered device metadata and platform.
- `sync_cursors`: per-client change cursor.
- `sync_changes`: append-only change feed for incremental sync.
- `sync_conflicts`: captured conflict records.

## Preview/Jobs
- `preview_assets`: generated thumbnails/previews and status.
- `jobs`: durable job metadata (queue refs in Redis).

## Security & Audit
- `audit_logs`: append-only security events, actor, IP, target, metadata JSONB.
- `login_attempts`: rate-limit and anomaly signals.
- `api_keys` (future plugin/service access).

## Indexing Strategy
- Unique `(parent_id, lower(name), deleted_at IS NULL)` logical name constraint.
- B-tree on `nodes(owner_user_id, parent_id)`.
- GIN on JSONB fields used for search metadata.
- Hash/B-tree on token hashes in `share_links`.
- Time-based indexes on `audit_logs(created_at)`, `jobs(next_run_at)`.

## Partitioning Plan
- Start unpartitioned for simplicity.
- Partition candidates at scale:
  - `audit_logs` by month.
  - `sync_changes` by month or tenant/user range.

## Migration Strategy
- SQL migrations with forward-only versioning.
- Bootstrap migrations:
  - `000001_init_extensions.sql`
  - `000002_identity.sql`
  - `000003_nodes.sql`
  - `000004_shares_acl.sql`
  - `000005_sync.sql`
  - `000006_previews_jobs_audit.sql`
