# ── Build stage ──────────────────────────────────
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=docker" \
    -o /stockyard ./cmd/stockyard

# ── Runtime stage ────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -h /home/stockyard stockyard
COPY --from=builder /stockyard /usr/local/bin/stockyard

# Data directory
RUN mkdir -p /data && chown stockyard:stockyard /data

ENV STOCKYARD_DATA_DIR=/data

# Default port
EXPOSE 4200
ENV PORT=4200

USER stockyard
ENTRYPOINT ["stockyard"]
