# Cloud Platform Architecture

## Vision
Build a modular, high-performance, self-hosted private cloud for individuals, families, and small teams. The design favors maintainability for a single core developer while leaving clear extension seams.

## Architecture Style
- **Modular monolith first** (single deployable backend binary) with strict internal boundaries.
- **Event-driven internals** for background jobs and extension hooks.
- **Clean Architecture layers**:
  - `domain`: pure business rules/entities/value objects
  - `application`: use-cases/services/orchestration
  - `infrastructure`: DB, Redis, storage, crypto, queue, preview tooling
  - `interfaces`: HTTP API, WebDAV, background workers

This gives operational simplicity now and an easier path to split services later.

## Core Runtime Components
- **Web App (Next.js)**: dashboard, file manager, sharing UI, admin UI.
- **API Server (Go)**:
  - REST/JSON API (`/api/v1`)
  - WebDAV endpoint (`/dav`)
  - Auth/session handling
  - Permission enforcement
- **Worker Process (Go)**:
  - preview/thumbnail generation
  - virus scanning hook (future)
  - async sync/background jobs
- **PostgreSQL**: metadata, auth, ACL, audit, shares, job metadata.
- **Redis**: queues, rate-limit counters, cache, resumable-upload state.
- **Storage backend**: local filesystem first; pluggable S3-compatible later.

## Domain Modules
- `identity`: users, auth, sessions, 2FA, roles.
- `files`: folders, files, versions, trash, tags, metadata.
- `sharing`: links, passwords, expiry, external access policies.
- `sync`: client registration, cursors, conflicts, delta feeds.
- `preview`: media analysis, thumbnail/preview lifecycle.
- `audit`: immutable security-relevant event stream.
- `platform`: quotas, settings, feature flags, plugin runtime.

## Request and Data Flow
1. Client requests API/WebDAV.
2. API gateway middleware handles auth, request ID, rate-limits, logging.
3. Application use-case validates permissions and quotas.
4. Metadata transaction persists in PostgreSQL.
5. Binary data streams to storage provider.
6. Domain events are emitted to Redis queue.
7. Worker consumes events and generates previews/index updates/audit events.

## Security Baseline
- TLS termination at reverse proxy (Nginx/Caddy).
- Strong password hashing (`argon2id`), optional TOTP 2FA.
- Short-lived access tokens + rotating refresh tokens.
- At-rest encryption by default via per-file data keys wrapped by master key.
- Audit logging for auth, permission changes, share creation/access.
- Secure headers, CSRF protection for browser flows, strict cookie flags.
- Rate limiting for auth/share endpoints and brute-force vectors.

## Deployment Topology
- **Local Dev**: Docker Compose (`api`, `worker`, `frontend`, `postgres`, `redis`).
- **Production (Debian)**:
  - `cloud-api.service`
  - `cloud-worker.service`
  - managed Postgres/Redis or local services
  - Nginx/Caddy reverse proxy
  - backups: DB dumps + storage snapshots + key material backups

## Observability
- Structured logs (`slog` JSON), correlation/request IDs.
- Metrics endpoint (`/metrics`, Prometheus format) in later phase.
- Health probes: liveness/readiness.
- Audit log and operational log separation.

## Key Tradeoffs
- **Modular monolith vs microservices**: monolith chosen for lower ops cost and faster iteration.
- **REST + WebDAV**: REST for modern clients, WebDAV for broad compatibility.
- **Server-side previews + optional E2EE vaults**: E2EE limits indexing/preview by design; documented behavior is explicit.
