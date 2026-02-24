# ═══════════════════════════════════════════════
# Stockyard — Multi-product Dockerfile
# Usage:
#   docker build --build-arg PRODUCT=costcap -t costcap .
#   docker run -e OPENAI_API_KEY=sk-... -p 4100:4100 costcap
# ═══════════════════════════════════════════════
FROM golang:1.22-alpine AS builder

ARG PRODUCT=stockyard
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app ./cmd/${PRODUCT}

# ── Final image: scratch + CA certs, <20MB ──
FROM scratch

ARG PRODUCT=stockyard

LABEL org.opencontainers.image.source="https://github.com/stockyard-dev/stockyard"
LABEL org.opencontainers.image.title="${PRODUCT}"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app /app

EXPOSE 4000 4100-4110 4200 4400-4500 4600-4901 5000-5500

ENTRYPOINT ["/app"]
