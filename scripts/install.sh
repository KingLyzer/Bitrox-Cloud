#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.yml"

APP_HOST=""
APP_PORT="11255"
FRONTEND_PORT=""
API_PORT="18080"
ADMIN_EMAIL=""
ADMIN_PASSWORD=""
PUBLIC_ORIGIN=""
SKIP_DEPS="0"
COMPOSE_MODE=""

info() {
  printf '[INFO] %s\n' "$1"
}

warn() {
  printf '[WARN] %s\n' "$1"
}

error() {
  printf '[ERROR] %s\n' "$1" >&2
}

usage() {
  cat <<'EOF'
Usage:
  sudo bash scripts/install.sh [options]

Options:
  --host <ip-or-hostname>      Public host/IP for URLs and CORS wiring.
  --port <port>                Public app port (frontend) (default: 11255).
  --frontend-port <port>       Explicit frontend port override (defaults to --port).
  --api-port <port>            Public backend API port (default: 18080).
  --public-origin <origin>     Extra frontend origin for CORS (e.g. http://tss.example.com:11255).
  --admin-email <email>        Bootstrap admin email override.
  --admin-password <password>  Bootstrap admin password override.
  --skip-deps                  Skip apt dependency installation checks.
  -h, --help                   Show this help.
EOF
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    error "This installer must run as root. Use: sudo bash scripts/install.sh ..."
    exit 1
  fi
}

detect_os_family() {
  if [[ ! -f /etc/os-release ]]; then
    error "Cannot detect OS (missing /etc/os-release)."
    exit 1
  fi
  . /etc/os-release
  case "${ID:-}" in
    debian|ubuntu) return 0 ;;
    *)
      case "${ID_LIKE:-}" in
        *debian*) return 0 ;;
      esac
      error "Unsupported OS '${ID:-unknown}'. This installer supports Debian/Ubuntu."
      exit 1
      ;;
  esac
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

ensure_docker_running() {
  if have_cmd systemctl; then
    systemctl enable --now docker >/dev/null 2>&1 || true
  fi
  if ! docker info >/dev/null 2>&1; then
    error "Docker daemon is not reachable. Check: systemctl status docker"
    exit 1
  fi
}

install_missing_packages_if_needed() {
  local need_update="0"
  local base_packages=()

  if ! have_cmd curl; then base_packages+=("curl"); fi
  if ! have_cmd jq; then base_packages+=("jq"); fi
  if ! have_cmd openssl; then base_packages+=("openssl"); fi
  if ! have_cmd ss; then base_packages+=("iproute2"); fi
  if [[ ${#base_packages[@]} -gt 0 ]]; then
    need_update="1"
  fi

  local install_docker="0"
  if ! have_cmd docker; then
    install_docker="1"
    need_update="1"
  fi

  export DEBIAN_FRONTEND=noninteractive
  if [[ "${need_update}" == "1" ]]; then
    info "Installing missing base packages: ${base_packages[*]:-<none>}"
    apt-get update -y
    if [[ ${#base_packages[@]} -gt 0 ]]; then
      apt-get install -y --no-install-recommends "${base_packages[@]}" ca-certificates
    fi
  fi

  if [[ "${install_docker}" == "1" ]]; then
    info "Docker command not found; installing distro Docker package (docker.io)."
    apt-get update -y
    apt-get install -y --no-install-recommends docker.io
  fi

  ensure_docker_running

  detect_compose_mode
  if [[ -z "${COMPOSE_MODE}" ]]; then
    info "Docker Compose not found; installing missing compose package."
    apt-get update -y
    if apt-get install -y --no-install-recommends docker-compose-plugin; then
      :
    else
      warn "docker-compose-plugin unavailable on this distro channel; using docker-compose package."
      apt-get install -y --no-install-recommends docker-compose
    fi
    detect_compose_mode
  fi

  if [[ -z "${COMPOSE_MODE}" ]]; then
    error "Docker Compose installation failed (no plugin or legacy docker-compose available)."
    exit 1
  fi
}

detect_default_host() {
  local host
  host="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ -z "${host}" ]]; then
    host="127.0.0.1"
  fi
  printf '%s' "${host}"
}

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

set_env_value() {
  local key="$1"
  local value="$2"
  local escaped
  escaped="$(escape_sed_replacement "${value}")"
  if grep -q "^${key}=" "${ENV_FILE}"; then
    sed -i "s/^${key}=.*/${key}=${escaped}/" "${ENV_FILE}"
  else
    printf '%s=%s\n' "${key}" "${value}" >> "${ENV_FILE}"
  fi
}

get_env_value() {
  local key="$1"
  local line
  line="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n 1 || true)"
  if [[ -z "${line}" ]]; then
    printf ''
    return 0
  fi
  printf '%s' "${line#*=}"
}

random_hex_32() {
  openssl rand -hex 32
}

random_password() {
  local value
  value="$(openssl rand -base64 36 | tr -dc 'A-Za-z0-9@#%+=._-' | head -c 28)"
  if [[ -z "${value}" ]]; then
    value="$(openssl rand -hex 16)"
  fi
  printf '%s' "${value}"
}

trim_spaces() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

normalize_origin() {
  local value
  value="$(trim_spaces "$1")"
  while [[ "${value}" == */ ]]; do
    value="${value%/}"
  done
  printf '%s' "${value}"
}

join_csv_unique() {
  local seen=","
  local out=()
  local candidate normalized key

  for candidate in "$@"; do
    normalized="$(normalize_origin "${candidate}")"
    if [[ -z "${normalized}" ]]; then
      continue
    fi
    key="$(printf '%s' "${normalized}" | tr '[:upper:]' '[:lower:]')"
    if [[ "${seen}" == *",${key},"* ]]; then
      continue
    fi
    seen="${seen}${key},"
    out+=("${normalized}")
  done

  local IFS=","
  printf '%s' "${out[*]}"
}

is_port_listening() {
  local port="$1"
  ss -ltn "( sport = :${port} )" | awk 'NR>1 {print $1}' | grep -q "LISTEN"
}

ensure_port_available_or_owned() {
  local port="$1"
  local label="$2"
  if ! have_cmd ss; then
    return 0
  fi

  if ! is_port_listening "${port}"; then
    return 0
  fi

  local frontend_cid api_cid
  frontend_cid="$(compose ps -q frontend 2>/dev/null || true)"
  api_cid="$(compose ps -q api 2>/dev/null || true)"
  if [[ -n "${frontend_cid}" || -n "${api_cid}" ]]; then
    warn "Port ${port} (${label}) is already listening; assuming existing stack ownership."
    return 0
  fi

  error "Port ${port} (${label}) is already in use."
  return 1
}

wait_for_service_health() {
  local service="$1"
  local timeout_seconds="$2"
  local started_at
  started_at="$(date +%s)"

  while true; do
    local cid
    cid="$(compose ps -q "${service}" 2>/dev/null || true)"
    if [[ -n "${cid}" ]]; then
      local status
      status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${cid}" 2>/dev/null || true)"
      if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
        info "Service '${service}' is ${status}."
        return 0
      fi
    fi

    local now
    now="$(date +%s)"
    if (( now - started_at > timeout_seconds )); then
      error "Timeout waiting for service '${service}' health."
      return 1
    fi
    sleep 2
  done
}

wait_for_http() {
  local url="$1"
  local timeout_seconds="$2"
  local started_at
  started_at="$(date +%s)"

  while true; do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      info "HTTP check passed: ${url}"
      return 0
    fi
    local now
    now="$(date +%s)"
    if (( now - started_at > timeout_seconds )); then
      error "Timeout waiting for HTTP endpoint: ${url}"
      return 1
    fi
    sleep 2
  done
}

print_failure_diagnostics() {
  warn "Installation failed. Collecting diagnostics..."
  if [[ -f "${ENV_FILE}" ]] && have_cmd docker && [[ -n "${COMPOSE_MODE}" ]]; then
    compose ps || true
    compose logs --tail=120 postgres redis api worker frontend || true
  fi
}

trap 'print_failure_diagnostics' ERR

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
    --public-origin)
      PUBLIC_ORIGIN="${2:-}"
      shift 2
      ;;
    --admin-email)
      ADMIN_EMAIL="${2:-}"
      shift 2
      ;;
    --admin-password)
      ADMIN_PASSWORD="${2:-}"
      shift 2
      ;;
    --skip-deps)
      SKIP_DEPS="1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      error "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

require_root

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  error "docker-compose.yml not found at ${COMPOSE_FILE}"
  exit 1
fi
if [[ ! -f "${PROJECT_ROOT}/.env.example" ]]; then
  error ".env.example not found at ${PROJECT_ROOT}/.env.example"
  exit 1
fi

detect_os_family

if [[ "${SKIP_DEPS}" != "1" ]]; then
  install_missing_packages_if_needed
else
  if ! have_cmd docker; then
    error "--skip-deps was used but docker is not installed."
    exit 1
  fi
  ensure_docker_running
  detect_compose_mode
  if [[ -z "${COMPOSE_MODE}" ]]; then
    error "--skip-deps was used but docker compose is not available."
    exit 1
  fi
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  info "Creating .env from .env.example"
  cp "${PROJECT_ROOT}/.env.example" "${ENV_FILE}"
fi

if [[ -z "${APP_HOST}" ]]; then
  APP_HOST="$(get_env_value APP_HOST)"
fi
if [[ -z "${APP_HOST}" ]]; then
  APP_HOST="$(detect_default_host)"
fi

if [[ -z "${FRONTEND_PORT}" ]]; then
  FRONTEND_PORT="${APP_PORT}"
else
  APP_PORT="${FRONTEND_PORT}"
fi

for candidate in "${APP_PORT}" "${FRONTEND_PORT}" "${API_PORT}"; do
  if ! [[ "${candidate}" =~ ^[0-9]+$ ]]; then
    error "Invalid port '${candidate}' (must be numeric)."
    exit 1
  fi
done

ensure_port_available_or_owned "${FRONTEND_PORT}" "frontend"
ensure_port_available_or_owned "${API_PORT}" "api"

set_env_value "APP_HOST" "${APP_HOST}"
set_env_value "APP_PORT" "${APP_PORT}"
set_env_value "FRONTEND_PORT" "${FRONTEND_PORT}"
set_env_value "API_PORT" "${API_PORT}"
set_env_value "NEXT_PUBLIC_API_BASE_URL" ""
set_env_value "API_PROXY_TARGET" "http://api:8080"

existing_cors_origins="$(get_env_value APP_CORS_ORIGINS)"
cors_candidates=(
  "http://${APP_HOST}:${FRONTEND_PORT}"
  "http://127.0.0.1:${FRONTEND_PORT}"
  "http://localhost:${FRONTEND_PORT}"
)
if [[ -n "${PUBLIC_ORIGIN}" ]]; then
  cors_candidates+=("${PUBLIC_ORIGIN}")
fi
if [[ -n "${existing_cors_origins}" ]]; then
  IFS=',' read -r -a existing_cors_array <<< "${existing_cors_origins}"
  cors_candidates+=("${existing_cors_array[@]}")
fi
merged_cors_origins="$(join_csv_unique "${cors_candidates[@]}")"
set_env_value "APP_CORS_ORIGINS" "${merged_cors_origins}"

set_env_value "APP_HTTP_ADDR" ":8080"
set_env_value "APP_ENV" "development"
set_env_value "APP_COOKIE_SECURE" "false"

if [[ -n "${ADMIN_EMAIL}" ]]; then
  set_env_value "APP_BOOTSTRAP_ADMIN_EMAIL" "${ADMIN_EMAIL}"
fi
if [[ -n "${ADMIN_PASSWORD}" ]]; then
  set_env_value "APP_BOOTSTRAP_ADMIN_PASSWORD" "${ADMIN_PASSWORD}"
else
  current_admin_password="$(get_env_value APP_BOOTSTRAP_ADMIN_PASSWORD)"
  if [[ -z "${current_admin_password}" || "${current_admin_password}" == "change-this-very-strong-password" ]]; then
    set_env_value "APP_BOOTSTRAP_ADMIN_PASSWORD" "$(random_password)"
  fi
fi

current_master_key="$(get_env_value APP_MASTER_KEY)"
if [[ -z "${current_master_key}" || "${current_master_key}" == "replace-with-at-least-32-characters-secret" || ${#current_master_key} -lt 32 ]]; then
  set_env_value "APP_MASTER_KEY" "$(random_hex_32)"
fi

current_jwt_key="$(get_env_value APP_JWT_SIGNING_KEY)"
if [[ -z "${current_jwt_key}" || "${current_jwt_key}" == "replace-with-at-least-32-characters-jwt-secret" || ${#current_jwt_key} -lt 32 ]]; then
  set_env_value "APP_JWT_SIGNING_KEY" "$(random_hex_32)"
fi

pg_db="$(get_env_value POSTGRES_DB)"
pg_user="$(get_env_value POSTGRES_USER)"
pg_password="$(get_env_value POSTGRES_PASSWORD)"
if [[ -z "${pg_db}" ]]; then pg_db="cloud"; fi
if [[ -z "${pg_user}" ]]; then pg_user="postgres"; fi
if [[ -z "${pg_password}" ]]; then pg_password="postgres"; fi
set_env_value "POSTGRES_DB" "${pg_db}"
set_env_value "POSTGRES_USER" "${pg_user}"
set_env_value "POSTGRES_PASSWORD" "${pg_password}"
set_env_value "APP_DATABASE_URL" "postgres://${pg_user}:${pg_password}@postgres:5432/${pg_db}?sslmode=disable"
set_env_value "APP_REDIS_URL" "redis://redis:6379/0"

info "Validating docker compose configuration."
compose config -q

info "Validating backend module lock files."
if [[ ! -f "${PROJECT_ROOT}/backend/go.mod" ]]; then
  error "backend/go.mod is missing."
  exit 1
fi
if [[ ! -f "${PROJECT_ROOT}/backend/go.sum" ]]; then
  error "backend/go.sum is missing. Deterministic builds require a committed go.sum."
  error "Run from backend module root: go mod tidy && go mod download"
  exit 1
elif [[ ! -s "${PROJECT_ROOT}/backend/go.sum" ]]; then
  error "backend/go.sum is empty. Deterministic builds require a populated go.sum."
  error "Run from backend module root: go mod tidy && go mod download"
  exit 1
fi

if [[ ! -f "${PROJECT_ROOT}/frontend/package-lock.json" ]]; then
  info "Generating frontend package-lock.json for deterministic dependency installs."
  docker run --rm \
    -v "${PROJECT_ROOT}/frontend:/app" \
    -w /app \
    node:20-bookworm \
    sh -lc "npm install --package-lock-only"
fi

info "Starting infrastructure services (postgres, redis)."
compose up -d postgres redis
wait_for_service_health "postgres" 180
wait_for_service_health "redis" 180

info "Applying database migrations in order."
compose --profile tools run --rm migrate

info "Building application images (api, worker, frontend)."
compose build --pull api worker frontend

info "Starting application services."
compose up -d api worker frontend
wait_for_service_health "api" 180
wait_for_http "http://127.0.0.1:${API_PORT}/health/live" 180
wait_for_http "http://127.0.0.1:${FRONTEND_PORT}" 240

effective_admin_email="$(get_env_value APP_BOOTSTRAP_ADMIN_EMAIL)"
effective_admin_password="$(get_env_value APP_BOOTSTRAP_ADMIN_PASSWORD)"

cat <<EOF

Installation completed successfully.

App URL (frontend): http://${APP_HOST}:${APP_PORT}
Frontend bind port: ${FRONTEND_PORT}
Backend API URL: http://${APP_HOST}:${API_PORT}
Backend health: http://${APP_HOST}:${API_PORT}/health/live

Bootstrap admin email: ${effective_admin_email}
Bootstrap admin password: ${effective_admin_password}

Security guidance:
- This bootstrap is HTTP-only for first bring-up.
- Before public internet exposure, place Nginx/Caddy with TLS in front and set:
  APP_ENV=production
  APP_COOKIE_SECURE=true
  APP_TRUSTED_PROXY_CIDRS=<your reverse proxy CIDR>

Next steps:
1) Run diagnostics: bash scripts/doctor.sh
2) Tail logs: docker compose logs -f api worker frontend
3) Rotate bootstrap admin password after first login.
EOF
