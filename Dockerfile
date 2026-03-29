# ── Build stage ──────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /stockyard ./cmd/stockyard/

# ── Runtime stage ────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl
COPY --from=build /stockyard /usr/local/bin/stockyard

EXPOSE 7749
VOLUME /data
ENV DATA_DIR=/data PORT=7749

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:7749/health || exit 1

ENTRYPOINT ["stockyard"]
