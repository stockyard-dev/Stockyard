FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

RUN addgroup -S stockyard && adduser -S stockyard -G stockyard -h /data

COPY stockyard /usr/local/bin/stockyard
RUN chmod +x /usr/local/bin/stockyard

RUN mkdir -p /data && chown stockyard:stockyard /data

ENV DATA_DIR=/data
ENV STOCKYARD_DB_PATH=/data/stockyard.db

USER stockyard
WORKDIR /data

ENTRYPOINT ["stockyard"]
