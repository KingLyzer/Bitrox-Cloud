# Frontend (Next.js + Tailwind)

## Runtime Model
- Production container builds static/server artifacts with `next build`
- Runtime starts with `next start`
- Dockerfile: [frontend/Dockerfile](C:/Projeler/Cloud/frontend/Dockerfile)

## API Wiring
Frontend reads:
- `NEXT_PUBLIC_API_BASE_URL`
- `API_PROXY_TARGET` (server-side Next.js rewrite target)

Default installer model (recommended):

```text
NEXT_PUBLIC_API_BASE_URL=
API_PROXY_TARGET=http://api:8080
```

Optional direct mode:

```text
NEXT_PUBLIC_API_BASE_URL=http://<APP_HOST>:<API_PORT>
```

## Local Dev (Optional)
```bash
cd frontend
npm install
npm run dev
```

## Compose Runtime
Default public app port is `11255` via `FRONTEND_PORT`.
