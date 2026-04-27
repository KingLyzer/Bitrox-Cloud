# WebDAV Integration

## Endpoint
- Mount path: `/dav/{user_scope}`
- Auth: app passwords/tokens preferred for DAV clients; basic auth over TLS only.

## Supported Methods
- `PROPFIND`, `OPTIONS`, `GET`, `HEAD`, `PUT`, `MKCOL`, `MOVE`, `COPY`, `DELETE`, `LOCK`, `UNLOCK`.

## Mapping Strategy
- WebDAV paths map to `nodes` in metadata.
- Binary operations stream through storage provider.
- ETags derive from current version hash and metadata revision.

## Compatibility Targets
- Windows network drive
- macOS Finder mount
- Linux GVFS/KIO
- mobile DAV clients

## Locking
- Soft lock records in Redis + DB fallback.
- Lease-based expiration to avoid deadlocks.

## Tradeoff
- Full RFC edge-case coverage is expensive; start with highly compatible subset validated against common clients.
