# Phase 2 Notes: Authentication and User Management

## What Was Added
- User domain and persistence model (`users` table).
- Session persistence model (`sessions` table).
- App password foundation (`app_passwords` table and repository).
- Quota policy table (`quota_policies`) and quota service scaffold.
- Admin bootstrap seeding at API startup from environment variables.
- Auth endpoints:
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/quota`

## Security Decisions
- Password hashing: Argon2id.
- Access token: short-lived JWT.
- Refresh token: opaque random token, persisted only as SHA-256 hash.
- Refresh rotation and reuse detection:
  - refresh always rotates on success.
  - replay of a rotated refresh token revokes the whole token family.
- Browser auth transport: secure HTTP-only cookies.
- CSRF strategy: double-submit token (`csrf` cookie + `X-CSRF-Token` header) with strict Origin allow-list validation on state-changing cookie-auth endpoints.

## Tradeoffs
- JWT access tokens are stateless and fast, but immediate invalidation requires short TTL and refresh checks.
- Token-family revocation on reuse is strict and can log users out in rare race/retry edge cases, but improves compromise containment.
- Quota usage reader is scaffolded (returns 0 until file metadata module lands), but quota policy model is ready.
- CSRF double-submit is simple and framework-agnostic, but frontend clients must read the CSRF cookie and mirror it in request headers.
