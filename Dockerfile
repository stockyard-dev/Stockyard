# ─── Build stage ─────────────────────────────────────────────────────────
FROM golang:1.22-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /stockyard ./cmd/stockyard/

# ─── Runtime stage ───────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /stockyard /usr/local/bin/stockyard

# Default data directory
RUN mkdir -p /data
VOLUME /data
ENV STOCKYARD_DATA_DIR=/data

# Default port
EXPOSE 4200
ENV PORT=4200

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:4200/health || exit 1

ENTRYPOINT ["stockyard"]
