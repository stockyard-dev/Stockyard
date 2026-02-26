# ─── Build stage ─────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /stockyard ./cmd/stockyard/

# ─── Runtime stage ───────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

# Non-root user
RUN addgroup -S stockyard && adduser -S stockyard -G stockyard

COPY --from=builder /stockyard /usr/local/bin/stockyard

# Data directory for SQLite
RUN mkdir -p /data && chown stockyard:stockyard /data
VOLUME /data

USER stockyard
WORKDIR /data

ENV STOCKYARD_DB_PATH=/data/stockyard.db
ENV STOCKYARD_LOG_FORMAT=json
ENV PORT=4200

EXPOSE 4200

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:4200/health || exit 1

ENTRYPOINT ["stockyard"]
