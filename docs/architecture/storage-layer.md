# Storage Abstraction Layer

## Goal
Support local filesystem first with seamless future S3-compatible backend adoption.

## Core Interface
```go
type ObjectStorage interface {
    Put(ctx context.Context, key string, r io.Reader, size int64, opts PutOptions) (ObjectMeta, error)
    Get(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectMeta, error)
    Delete(ctx context.Context, key string) error
    Stat(ctx context.Context, key string) (ObjectMeta, error)
    Copy(ctx context.Context, srcKey, dstKey string) error
}
```

## Design Decisions
- Storage keys are immutable content pointers, not user paths.
- File/folder names live in DB metadata (`nodes`), never in storage backend paths.
- Versioning creates new storage objects; metadata points active version.
- Envelope encryption is performed before `Put` by crypto service.

## Local Filesystem Backend
- Root path configured by `STORAGE_LOCAL_ROOT`.
- Directory sharding by hash prefix (`ab/cd/<key>`) to avoid huge dirs.
- Atomic writes via temp file + rename.
- Streaming reads to minimize memory pressure.

## Future S3 Backend
- Uses same interface and key format.
- Multipart upload for large files.
- Consistency handling with metadata transaction patterns.

## Consistency Pattern
- Two-phase app-level flow:
  1. Create upload/session metadata.
  2. Store object.
  3. Finalize DB transaction with object metadata.
- Recovery jobs reconcile orphaned objects/metadata.
