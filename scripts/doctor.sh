#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.yml"

FAILURES=0
APP_HOST=""
APP_PORT=""
FRONTEND_PORT=""
API_PORT=""
COMPOSE_MODE=""

info() {
  printf '[INFO] %s\n' "$1"
}

ok() {
  printf '[OK] %s\n' "$1"
}

fail() {
  printf '[FAIL] %s\n' "$1"
  FAILURES=$((FAILURES + 1))
}

usage() {
  cat <<'EOF'
Usage:
  bash scripts/doctor.sh [--host <ip-or-hostname>] [--port <app-port>] [--frontend-port <port>] [--api-port <port>]
EOF
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

detect_compose_mode() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_MODE="plugin"
    return
  fi
  if have_cmd docker-compose; then
    COMPOSE_MODE="legacy"
    return
  fi
  COMPOSE_MODE=""
}

compose() {
  if [[ "${COMPOSE_MODE}" == "plugin" ]]; then
    docker compose --project-directory "${PROJECT_ROOT}" --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
  else
    docker-compose --project-directory "${PROJECT_ROOT}" --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
  fi
}

env_get() {
  local key="$1"
  local line
  line="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n 1 || true)"
  if [[ -z "${line}" ]]; then
    printf ''
    return 0
  fi
  printf '%s' "${line#*=}"
}

check_cmd() {
  local cmd="$1"
  if have_cmd "${cmd}"; then
    ok "Command available: ${cmd}"
  else
    fail "Missing command: ${cmd}"
  fi
}

check_required_env_key() {
  local key="$1"
  local value
  value="$(env_get "${key}")"
  if [[ -z "${value}" ]]; then
    fail "Missing env key in .env: ${key}"
  else
    ok "Env key present: ${key}"
  fi
}

check_no_placeholder() {
  local key="$1"
  local bad_value="$2"
  local value
  value="$(env_get "${key}")"
  if [[ "${value}" == "${bad_value}" ]]; then
    fail "Env key still uses placeholder value: ${key}"
  else
    ok "Env key configured: ${key}"
  fi
}

check_service_running() {
  local service="$1"
  local cid
  cid="$(compose ps -q "${service}" || true)"
  if [[ -z "${cid}" ]]; then
    fail "Service not found/running: ${service}"
    return
  fi
  local state
  state="$(docker inspect --format '{{.State.Status}}' "${cid}" 2>/dev/null || true)"
  if [[ "${state}" == "running" ]]; then
    ok "Service running: ${service}"
  else
    fail "Service state for ${service}: ${state:-unknown}"
  fi
}

check_service_healthy() {
  local service="$1"
  local cid
  cid="$(compose ps -q "${service}" || true)"
  if [[ -z "${cid}" ]]; then
    fail "Service missing for health check: ${service}"
    return
  fi
  local status
  status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${cid}" 2>/dev/null || true)"
  if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
    ok "Service healthy: ${service} (${status})"
  else
    fail "Service unhealthy: ${service} (${status:-unknown})"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      APP_HOST="${2:-}"
      shift 2
      ;;
    --port)
      APP_PORT="${2:-}"
      shift 2
      ;;
    --frontend-port)
      FRONTEND_PORT="${2:-}"
      shift 2
      ;;
    --api-port)
      API_PORT="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

info "Checking basic prerequisites."
check_cmd docker
check_cmd curl
check_cmd ss

detect_compose_mode
if [[ "${COMPOSE_MODE}" == "plugin" ]]; then
  ok "Docker Compose plugin available"
elif [[ "${COMPOSE_MODE}" == "legacy" ]]; then
  ok "Legacy docker-compose available"
else
  fail "Docker Compose is not available"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  fail "Missing .env file at ${ENV_FILE}"
  exit 1
fi
ok ".env file found"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  fail "Missing docker-compose.yml at ${COMPOSE_FILE}"
  exit 1
fi
ok "docker-compose.yml found"

info "Validating compose configuration."
if compose config -q >/dev/null 2>&1; then
  ok "docker compose config is valid"
else
  fail "docker compose config validation failed"
fi

info "Checking required environment values."
check_required_env_key "APP_HOST"
check_required_env_key "APP_PORT"
check_required_env_key "FRONTEND_PORT"
check_required_env_key "API_PORT"
check_required_env_key "APP_DATABASE_URL"
check_required_env_key "APP_REDIS_URL"
check_required_env_key "APP_STORAGE_LOCAL_ROOT"
check_required_env_key "APP_BOOTSTRAP_ADMIN_EMAIL"
check_no_placeholder "APP_MASTER_KEY" "replace-with-at-least-32-characters-secret"
check_no_placeholder "APP_JWT_SIGNING_KEY" "replace-with-at-least-32-characters-jwt-secret"

if [[ -z "${APP_HOST}" ]]; then APP_HOST="$(env_get APP_HOST)"; fi
if [[ -z "${APP_HOST}" ]]; then APP_HOST="127.0.0.1"; fi
if [[ -z "${APP_PORT}" ]]; then APP_PORT="$(env_get APP_PORT)"; fi
if [[ -z "${APP_PORT}" ]]; then APP_PORT="11255"; fi
if [[ -z "${FRONTEND_PORT}" ]]; then FRONTEND_PORT="$(env_get FRONTEND_PORT)"; fi
if [[ -z "${FRONTEND_PORT}" ]]; then FRONTEND_PORT="${APP_PORT}"; fi
if [[ -z "${API_PORT}" ]]; then API_PORT="$(env_get API_PORT)"; fi
if [[ -z "${API_PORT}" ]]; then API_PORT="18080"; fi

info "Current compose state."
compose ps || true

info "Checking container states."
check_service_running "postgres"
check_service_running "redis"
check_service_running "api"
check_service_running "worker"
check_service_running "frontend"

info "Checking container health."
check_service_healthy "postgres"
check_service_healthy "redis"
check_service_healthy "api"
check_service_healthy "frontend"

info "Checking host port reachability."
if ss -ltn "( sport = :${FRONTEND_PORT} )" | awk 'NR>1 {print $1}' | grep -q "LISTEN"; then
  ok "Frontend port is listening on host: ${FRONTEND_PORT}"
else
  fail "Frontend port is not listening on host: ${FRONTEND_PORT}"
fi
if ss -ltn "( sport = :${API_PORT} )" | awk 'NR>1 {print $1}' | grep -q "LISTEN"; then
  ok "API port is listening on host: ${API_PORT}"
else
  fail "API port is not listening on host: ${API_PORT}"
fi

info "Checking backend and frontend HTTP reachability."
if curl -fsS "http://127.0.0.1:${API_PORT}/health/live" >/dev/null; then
  ok "Backend health endpoint reachable"
else
  fail "Backend health endpoint not reachable at 127.0.0.1:${API_PORT}/health/live"
fi
if curl -fsS "http://127.0.0.1:${API_PORT}/health/ready" >/dev/null; then
  ok "Backend ready endpoint reachable"
else
  fail "Backend ready endpoint not reachable at 127.0.0.1:${API_PORT}/health/ready"
fi
if curl -fsS "http://127.0.0.1:${FRONTEND_PORT}" >/dev/null; then
  ok "Frontend endpoint reachable"
else
  fail "Frontend endpoint not reachable at 127.0.0.1:${FRONTEND_PORT}"
fi

info "Checking storage path writability inside API container."
if compose exec -T api sh -lc 'mkdir -p "$APP_STORAGE_LOCAL_ROOT/.doctor" && test -w "$APP_STORAGE_LOCAL_ROOT" && rm -rf "$APP_STORAGE_LOCAL_ROOT/.doctor"' >/dev/null 2>&1; then
  ok "Storage path writable from API container"
else
  fail "Storage path not writable from API container"
fi

info "Checking database connectivity and migration footprint."
pg_user="$(env_get POSTGRES_USER)"
pg_db="$(env_get POSTGRES_DB)"
if [[ -z "${pg_user}" ]]; then pg_user="postgres"; fi
if [[ -z "${pg_db}" ]]; then pg_db="cloud"; fi

if compose exec -T postgres pg_isready -U "${pg_user}" -d "${pg_db}" >/dev/null 2>&1; then
  ok "PostgreSQL readiness check passed"
else
  fail "PostgreSQL readiness check failed"
fi

if compose exec -T redis redis-cli ping 2>/dev/null | grep -q "PONG"; then
  ok "Redis ping check passed"
else
  fail "Redis ping check failed"
fi

table_count="$(compose exec -T postgres psql -U "${pg_user}" -d "${pg_db}" -tAc "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','sessions','nodes','file_versions','upload_sessions','upload_chunks');" 2>/dev/null || true)"
if [[ "${table_count}" == "6" ]]; then
  ok "Core migration tables present"
else
  fail "Core migration tables missing (expected 6, got '${table_count:-none}')"
fi

phase4_table_count="$(compose exec -T postgres psql -U "${pg_user}" -d "${pg_db}" -tAc "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('app_settings','calendar_events','calendar_event_reminders','notifications','shares','share_access_grants');" 2>/dev/null || true)"
if [[ "${phase4_table_count}" == "6" ]]; then
  ok "Phase4 tables present"
else
  fail "Phase4 tables missing (expected 6, got '${phase4_table_count:-none}')"
fi

if [[ "${FAILURES}" -ne 0 ]]; then
  printf '\nDoctor checks completed with %d failure(s).\n' "${FAILURES}"
  printf 'Inspect logs with:\n'
  printf '  docker compose logs --tail=120 api worker postgres redis frontend\n'
  exit 1
fi

printf '\nAll doctor checks passed.\n'
printf 'App URL:      http://%s:%s\n' "${APP_HOST}" "${APP_PORT}"
printf 'Frontend URL: http://%s:%s\n' "${APP_HOST}" "${FRONTEND_PORT}"
printf 'API URL:      http://%s:%s\n' "${APP_HOST}" "${API_PORT}"
