# Backend (Go)

Backend module root is `backend/` with:

```text
module cloud/backend
```

## Runtime Model
- API runs as compiled binary from `./cmd/api`
- Worker runs as compiled binary from `./cmd/worker`
- Production container build is defined in:
  - [backend/Dockerfile](C:/Projeler/Cloud/backend/Dockerfile)

No production deployment path depends on `go run`.

## Dependency Hygiene
Use module root:

```bash
cd backend
go mod tidy
```

Deployment installer requires `backend/go.mod` and checks `backend/go.sum`; missing/empty `go.sum` is warned (not blocked) so containerized builds can still proceed.

## Migrations
SQL migrations are in `backend/migrations` and applied in lexical order by compose `migrate` service.

Manual run:

```bash
docker compose --profile tools run --rm migrate
```
