# Sync Architecture (Desktop/Mobile)

## Goals
- Reliable bi-directional sync.
- Efficient delta sync.
- Deterministic conflict handling.

## Model
- Each client registers as `sync_client`.
- Server maintains append-only `sync_changes`.
- Client tracks cursor and local index.

## APIs
- Initial scan and baseline index.
- Delta pull by cursor.
- Batched upload/change push.
- Conflict resolution endpoint.

## Conflict Strategy
- Detect by content hash/version mismatch.
- Auto-resolve non-overlapping metadata changes.
- Content conflicts generate sibling copy naming:
  - `filename (conflict <device> <timestamp>).ext`

## Background Jobs
- Mobile camera backup uploads in resumable chunks.
- Retry with exponential backoff and jitter.
- Opportunistic compression and battery/network heuristics (client side).

## Reliability
- Idempotency tokens for change submissions.
- At-least-once delivery semantics with dedupe keys.
- Periodic reconciliation scan endpoint for drift recovery.
