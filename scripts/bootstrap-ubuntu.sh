#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="/var/www/bitrocloud"
REPO_URL=""
BRANCH="main"
APP_HOST=""
APP_PORT="11255"
API_PORT="18080"
PUBLIC_ORIGIN=""

info() {
  printf '[INFO] %s\n' "$1"
}

error() {
  printf '[ERROR] %s\n' "$1" >&2
}

usage() {
  cat <<'EOF'
Usage:
  sudo bash bootstrap-ubuntu.sh --repo <git-repo-url> [options]

Options:
  --repo <url>            Git repository URL (required).
  --branch <name>         Git branch (default: main).
  --host <ip-or-hostname> Public host/IP for frontend URL wiring.
  --port <port>           Frontend/app port (default: 11255).
  --api-port <port>       Public API port (default: 18080).
  --public-origin <url>   Extra CORS origin, optional.
  -h, --help              Show help.
EOF
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    error "Run as root: sudo bash bootstrap-ubuntu.sh ..."
    exit 1
  fi
}

install_base_tools() {
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get install -y --no-install-recommends git ca-certificates curl
}

clone_or_update_repo() {
  mkdir -p /var/www

  if [[ -d "${TARGET_DIR}/.git" ]]; then
    info "Repository exists at ${TARGET_DIR}, updating."
    git -C "${TARGET_DIR}" fetch --all --prune
    git -C "${TARGET_DIR}" checkout "${BRANCH}"
    git -C "${TARGET_DIR}" pull --ff-only origin "${BRANCH}"
    return
  fi

  if [[ -d "${TARGET_DIR}" ]] && [[ -n "$(ls -A "${TARGET_DIR}" 2>/dev/null || true)" ]]; then
    error "${TARGET_DIR} exists and is not empty (and not a git repo). Clean it first."
    exit 1
  fi

  info "Cloning repository into ${TARGET_DIR}"
  rm -rf "${TARGET_DIR}"
  git clone --branch "${BRANCH}" "${REPO_URL}" "${TARGET_DIR}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_URL="${2:-}"
      shift 2
      ;;
    --branch)
      BRANCH="${2:-}"
      shift 2
      ;;
    --host)
      APP_HOST="${2:-}"
      shift 2
      ;;
    --port)
      APP_PORT="${2:-}"
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

if [[ -z "${REPO_URL}" ]]; then
  error "--repo is required."
  usage
  exit 1
fi

install_base_tools
clone_or_update_repo

info "Running project installer from ${TARGET_DIR}"
install_args=(
  --port "${APP_PORT}"
  --api-port "${API_PORT}"
  --admin-email "admin@example.com"
  --admin-password "Passw0rd!123"
)
if [[ -n "${APP_HOST}" ]]; then
  install_args+=(--host "${APP_HOST}")
fi
if [[ -n "${PUBLIC_ORIGIN}" ]]; then
  install_args+=(--public-origin "${PUBLIC_ORIGIN}")
fi
bash "${TARGET_DIR}/scripts/install.sh" "${install_args[@]}"

info "Bootstrap finished."
info "Project path: ${TARGET_DIR}"
