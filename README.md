# Cloud Platform

Self-hosted private cloud platform with production-style containers:
- `backend` (Go API binary)
- `worker` (Go worker binary)
- `frontend` (Next.js build + start)
- `postgres`
- `redis`

## One-Command Install (Debian/Ubuntu)
On a fresh Ubuntu/Debian server, install directly from GitHub into `/var/www/bitrocloud`:

```bash
curl -fsSL https://raw.githubusercontent.com/<your-org>/<your-repo>/main/scripts/bootstrap-ubuntu.sh -o /tmp/bootstrap-ubuntu.sh
sudo bash /tmp/bootstrap-ubuntu.sh --repo https://github.com/<your-org>/<your-repo>.git --branch main --host <server-ip-or-domain> --port 11255
```

This flow:
- clones/updates the project at `/var/www/bitrocloud`
- installs missing dependencies
- configures firewall rules (`ufw allow 22`, app port, API port)
- installs and starts Docker/Compose services
- runs DB migrations
- builds and starts API/worker/frontend
- waits for health endpoints
- prepares default bootstrap admin credentials:
  - email: `admin@example.com`
  - password: `Passw0rd!123`

The installer:
1. Detects Docker/Compose and avoids unnecessary Docker reinstall if already present.
2. Installs only missing prerequisites when needed.
3. Creates/updates `.env` safely.
4. Generates strong secrets if placeholders remain.
5. Verifies backend module lock files (`backend/go.mod` and non-empty `backend/go.sum` are required).
6. Starts infra services, runs migrations, builds/starts app services.
7. Waits for backend health and frontend reachability.
8. Prints final URLs and bootstrap admin credentials.

If you already cloned the repo manually, run:

```bash
cd /var/www/bitrocloud
sudo bash scripts/install.sh --host <server-ip-or-domain> --port 11255
```

## Doctor / Verification
Run:

```bash
bash scripts/doctor.sh
```

Doctor verifies:
- `.env` core values and non-placeholder secrets
- compose config validity
- running/healthy containers
- API health endpoint and frontend reachability
- DB readiness + migration footprint
- storage path writability from API container

## Port Model (Default)
- Public app URL (frontend): `APP_PORT=11255`
- Frontend bind: `FRONTEND_PORT=11255` (defaults to app port)
- Public API URL: `API_PORT=18080`
- Backend internal listen: `APP_HTTP_ADDR=:8080`

Default expected URLs:
- App: `http://<host>:11255`
- API health/live: `http://<host>:18080/health/live`
- API health/ready: `http://<host>:18080/health/ready`

## Configuration
Single `.env` is the runtime source of truth.

Most important keys:
- `APP_HOST`
- `APP_PORT`
- `FRONTEND_PORT`
- `API_PORT`
- `NEXT_PUBLIC_API_BASE_URL`
- `APP_CORS_ORIGINS`
- `APP_DATABASE_URL`
- `APP_REDIS_URL`

## Common Commands
Start/restart stack:

```bash
docker compose up -d
```

Re-run migrations:

```bash
docker compose --profile tools run --rm migrate
```

View logs:

```bash
docker compose logs --tail=150 api worker frontend postgres redis
```

## Frontend Phase 4 (File Manager UI)
The web UI now includes:
- authenticated app shell (sidebar + topbar + responsive layout)
- file browser (breadcrumb, list/grid, sort, selection, context actions)
- modal flows (create folder, rename, delete confirm, move)
- drag-and-drop and file-picker uploads with progress queue
- details side panel foundation (preview/share/version/trash hooks)
- admin panel for owner/admin users (user create/update/quota/password reset)
- light/dark theme support

### Run Frontend Locally
```bash
cd frontend
npm install
npm run dev
```

Required env for API integration:
- `NEXT_PUBLIC_API_BASE_URL`

### Run Full Stack (Docker)
```bash
docker compose up -d
```

Then open:
- Frontend: `http://<host>:<FRONTEND_PORT>`
- API health/live: `http://<host>:<API_PORT>/health/live`
- API health/ready: `http://<host>:<API_PORT>/health/ready`

### Manual Phase 4 Smoke Test
1. Login with bootstrap admin.
2. Create folder, rename it, move it, then delete it.
3. Upload files with file picker.
4. Upload files with drag and drop.
5. Verify upload progress cards and final file visibility.
6. Open details panel by selecting a node.
7. Switch list/grid mode, test sorting and search.
8. Open Admin Panel:
   - create user
   - change role/quota/active state
   - reset user password

## Troubleshooting
### Docker already installed conflicts
- Installer checks first and does not blindly force Docker CE packages.
- If Docker exists but Compose is missing, installer adds only compose package.

### Port already in use
- Installer checks frontend/API host ports before startup.
- Change with `--port`, `--frontend-port`, `--api-port`.

### Missing `go.sum` / dependency lock issues
- Ensure `backend/go.sum` is present and committed for deterministic builds.
- Installer fails fast if `go.sum` is missing/empty.
- Regenerate from module root: `cd backend && go mod tidy && go mod download`.

### Backend unhealthy
1. `docker compose ps`
2. `docker compose logs --tail=200 api`
3. `docker compose --profile tools run --rm migrate`

### Frontend cannot reach API
- Check `.env`:
  - Preferred proxy mode:
    - `NEXT_PUBLIC_API_BASE_URL=`
    - `API_PROXY_TARGET=http://api:8080`
  - Optional direct mode:
    - `NEXT_PUBLIC_API_BASE_URL=http://<host>:<API_PORT>`
  - `APP_CORS_ORIGINS=http://<host>:<FRONTEND_PORT>`
- Rebuild frontend:
  - `docker compose build frontend && docker compose up -d frontend`

## HTTP Bootstrap Note
Bootstrap is HTTP-only by default for first bring-up.

Before public internet exposure:
1. Put Nginx/Caddy in front with TLS.
2. Set:
   - `APP_ENV=production`
   - `APP_COOKIE_SECURE=true`
   - `APP_TRUSTED_PROXY_CIDRS=<proxy CIDR>`
