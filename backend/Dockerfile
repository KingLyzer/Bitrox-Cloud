FROM golang:1.24-bookworm AS deps

WORKDIR /src
ENV GOFLAGS=-mod=readonly

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

FROM deps AS builder

WORKDIR /src
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM debian:bookworm-slim AS runtime-base

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
ENV APP_HTTP_ADDR=:8080

FROM runtime-base AS api
COPY --from=builder /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM runtime-base AS worker
COPY --from=builder /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
