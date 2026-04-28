#!/bin/bash
set -euo pipefail

DB_URL="${APP_DATABASE_URL:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNING_IN_CONTAINER="0"

if [[ -f "/.dockerenv" ]]; then
  RUNNING_IN_CONTAINER="1"
fi

if [[ -z "${DB_URL}" ]]; then
  echo "[ERROR] APP_DATABASE_URL is required for migration runner." >&2
  exit 1
fi

if [[ -n "${MIGRATIONS_DIR:-}" && -d "${MIGRATIONS_DIR:-}" ]]; then
  RESOLVED_MIGRATIONS_DIR="${MIGRATIONS_DIR}"
elif [[ -d "/migrations" ]]; then
  RESOLVED_MIGRATIONS_DIR="/migrations"
elif [[ -d "${SCRIPT_DIR}/../backend/migrations" ]]; then
  RESOLVED_MIGRATIONS_DIR="${SCRIPT_DIR}/../backend/migrations"
else
  echo "[ERROR] migrations directory not found (checked: MIGRATIONS_DIR, /migrations, ${SCRIPT_DIR}/../backend/migrations)." >&2
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "[ERROR] psql command is required but not found." >&2
  exit 1
fi

echo "[INFO] Migration runner started."
echo "[INFO] running_in_container=${RUNNING_IN_CONTAINER}"
echo "[INFO] migrations_dir=${RESOLVED_MIGRATIONS_DIR}"

psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);"

# Runtime safety guards for older schemas before admin bootstrap.
psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "
ALTER TABLE IF EXISTS users ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS quota_bytes BIGINT DEFAULT 5368709120;
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS preferred_language TEXT NULL;
"

mapfile -t files < <(find "${RESOLVED_MIGRATIONS_DIR}" -maxdepth 1 -type f -name '*.up.sql' -printf '%f\n' | LC_ALL=C sort)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "[WARN] No migration files found in ${RESOLVED_MIGRATIONS_DIR}"
  exit 0
fi

for name in "${files[@]}"; do
  file="${RESOLVED_MIGRATIONS_DIR}/${name}"
  already_applied="$(psql "${DB_URL}" -tAc "SELECT 1 FROM schema_migrations WHERE filename='${name}' LIMIT 1;")"
  if [[ "${already_applied}" == "1" ]]; then
    echo "[INFO] Skipping already applied migration: ${name}"
    continue
  fi

  echo "[INFO] Applying migration: ${name}"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${file}"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations (filename) VALUES ('${name}') ON CONFLICT (filename) DO NOTHING;"
done

# Final safety guard in case historical migrations used nullable/default-less fields.
psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "
CREATE EXTENSION IF NOT EXISTS pgcrypto;
ALTER TABLE IF EXISTS users ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS quota_bytes BIGINT DEFAULT 5368709120;
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS preferred_language TEXT NULL;
"

echo "[INFO] Migration runner completed successfully."
