# Files Application Service

Phase 3 orchestration for file/folder lifecycle:
- folder CRUD orchestration with hierarchy validation
- safe rename/move semantics
- soft-delete foundation for future trash restore
- resumable upload sessions with chunk writes
- upload finalize flow (chunk composition + MIME/hash analysis + metadata commit)
- download stream orchestration
- expired upload reaper orchestration (abort + staged object cleanup retry)

Design notes:
- logical tree is driven by stable node IDs + parent relationships
- storage keys are opaque and decoupled from user-visible paths
- quota is enforced at finalize/commit transition, not chunk ingest time
- staged upload budget (`active sessions`, `staged bytes`) is enforced pre-create and on chunk writes
- chunk writes are immutable/idempotent by `(chunk_index, size, sha256)` contract
- immutable file version rows are created on each finalized write/update
