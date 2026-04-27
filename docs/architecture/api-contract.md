# API Contract (v1)

Base path: `/api/v1`  
Auth: `Authorization: Bearer <access-token>` or secure cookie session for web app.

## Auth
- `POST /auth/register` (optional initial self-signup mode)
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/mfa/totp/setup`
- `POST /auth/mfa/totp/verify`
- `POST /auth/mfa/totp/disable`

## Users/Admin
- `GET /me`
- `PATCH /me/preferences`
- `GET /admin/users`
- `POST /admin/users`
- `PATCH /admin/users/{id}`
- `POST /admin/users/{id}/reset-password`

## Files and Folders
- `GET /nodes?parent_id=&cursor=`
- `POST /nodes/folders`
- `PATCH /nodes/{id}` (rename/move/tag metadata)
- `DELETE /nodes/{id}` (to trash)
- `POST /nodes/{id}/restore`
- `GET /nodes/{id}/download`
- `GET /nodes/{id}/versions`
- `POST /nodes/{id}/versions/{version_id}/restore`

## Uploads (Chunked/Resumable)
- `POST /uploads` (create upload session)
- `PUT /uploads/{id}/chunks/{index}`
- `POST /uploads/{id}/complete`
- `GET /uploads/{id}`
- `DELETE /uploads/{id}`

## Shares
- `POST /shares/links`
- `GET /shares/links`
- `PATCH /shares/links/{id}`
- `DELETE /shares/links/{id}`
- `GET /public/s/{token}` (metadata/public preview policy)
- `GET /public/s/{token}/download`

## Quota & Storage
- `GET /quota`
- `GET /storage/usage`

## Sync
- `POST /sync/clients/register`
- `POST /sync/clients/{id}/heartbeat`
- `GET /sync/changes?cursor=...`
- `POST /sync/changes/ack`
- `POST /sync/conflicts/{id}/resolve`

## Audit
- `GET /audit/events` (admin/security roles)

## System
- `GET /health/live`
- `GET /health/ready`
- `GET /version`

## Error Envelope
```json
{
  "error": {
    "code": "invalid_request",
    "message": "Human-readable message",
    "details": {}
  },
  "request_id": "..."
}
```

## Response Principles
- Cursor-based pagination for list endpoints.
- ETag + conditional requests for sync and metadata endpoints.
- Idempotency key support for sensitive write endpoints.
