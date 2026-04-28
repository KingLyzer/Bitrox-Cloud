#!/bin/bash
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
DB_URL="${APP_DATABASE_URL:-}"

if [[ -z "${DB_URL}" ]]; then
  echo "[ERROR] APP_DATABASE_URL is required for migration runner." >&2
  exit 1
fi

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "[ERROR] migrations directory not found: ${MIGRATIONS_DIR}" >&2
  exit 1
fi

psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "
CREATE TABLE IF NOT EXISTS schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);"

mapfile -t files < <(find "${MIGRATIONS_DIR}" -maxdepth 1 -type f -name '*.up.sql' | sort)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "[WARN] No migration files found in ${MIGRATIONS_DIR}"
  exit 0
fi

for file in "${files[@]}"; do
  name="$(basename "${file}")"
  already_applied="$(psql "${DB_URL}" -tAc "SELECT 1 FROM schema_migrations WHERE filename='${name}' LIMIT 1;")"
  if [[ "${already_applied}" == "1" ]]; then
    echo "[INFO] Skipping already applied migration: ${name}"
    continue
  fi

  echo "[INFO] Applying migration: ${name}"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${file}"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations (filename) VALUES ('${name}');"
done

echo "[INFO] Migration runner completed successfully."
