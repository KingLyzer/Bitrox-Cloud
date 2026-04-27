# Plugin Architecture Foundation

## Goals
- Extend features without core rewrites.
- Keep core secure and predictable.

## Plugin Types
- **Internal modules** (compiled with core, first phase).
- **External plugins** (future, process/plugin SDK boundary).

## Extension Points
- Domain events (`file.uploaded`, `share.created`, `user.logged_in`).
- HTTP route injection under `/api/v1/plugins/{name}`.
- UI extension manifests (frontend widgets/pages in later phase).
- Scheduled tasks/hooks.

## Runtime Contract
- Manifest:
  - plugin id/name/version
  - required permissions/scopes
  - enabled/disabled state
- Lifecycle:
  - `Init(ctx, deps)`
  - `Start(ctx)`
  - `Stop(ctx)`

## Safety Model
- Permission-scoped plugin APIs.
- Config namespacing per plugin.
- Audit trails for plugin actions.

## Future Direction
- WASM or sidecar sandbox for untrusted third-party plugins.
