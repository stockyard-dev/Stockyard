# Stage 1: Build
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /stockyard ./cmd/stockyard/

# Stage 2: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl
COPY --from=builder /stockyard /usr/local/bin/stockyard
RUN chmod +x /usr/local/bin/stockyard

CMD ["sh", "-c", "echo START PORT=$PORT HOME=$HOME PWD=$PWD && ls -la /usr/local/bin/stockyard && stockyard 2>&1"]
