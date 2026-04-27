# Thumbnail and Preview Pipeline

## Objectives
- Fast browsing with low-latency thumbnails.
- Scalable preview generation for large media.
- Graceful degradation on unsupported formats.

## Flow
1. File finalize emits `file.uploaded` event.
2. Worker classifies media type and queues derivative tasks.
3. Derivatives are written to preview storage namespace.
4. `preview_assets` status updates (`pending`, `ready`, `failed`).
5. API serves signed/internal URLs with caching headers.

## Tooling
- Images: `libvips` for fast thumbnails.
- PDFs: `pdftoppm`/`poppler` first page preview.
- Videos: `ffmpeg` frame extraction + optional short proxy clip.

## Performance Strategies
- Tiered derivatives (`xs`, `sm`, `md`, `lg`).
- Timeouts and CPU quotas per job.
- Queue prioritization for currently viewed files.
- Debounced generation to avoid duplicate work.

## Security
- Never execute untrusted content.
- MIME sniff + magic-byte validation before processing.
- Resource limits to avoid decompression bombs.
