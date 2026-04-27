# Authentication and Authorization Model

## Authentication
- Password-based login with `argon2id`.
- Access token: short-lived JWT (5-15 min).
- Refresh token: opaque, rotating, stored hashed in DB.
- Browser sessions prefer secure `HttpOnly` cookies.
- Optional TOTP 2FA per user.

## Session Security
- Refresh token rotation with reuse detection.
- Device/session listing and selective revocation.
- Session-bound metadata: user-agent fingerprint, IP, last seen.

## Authorization
- Base RBAC roles:
  - `owner`: full instance control
  - `admin`: user/admin operations except ownership transfer
  - `member`: standard personal/team operations
  - `viewer`: read-only where explicitly granted
- Fine-grained ACL per node for sharing/collaboration.

## Permission Checks
- Central policy engine in `application/policy`.
- Explicit actions:
  - `node.read`
  - `node.write`
  - `node.delete`
  - `node.share`
  - `admin.users.manage`
  - `audit.read`

## Quotas
- Effective quota resolved as:
  - user override quota
  - else role/group policy
  - else instance default
- Writes enforce hard limits at finalize stage.

## E2EE Vaults
- Optional folder mode where server stores encrypted blobs/metadata subset.
- Server-side previews/search disabled or limited by design for E2EE content.
