FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -o /bin/fossil ./cmd/fossil

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /bin/fossil /usr/local/bin/fossil

EXPOSE 8080
ENTRYPOINT ["fossil"]
