FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

RUN addgroup -S stockyard && adduser -S stockyard -G stockyard

COPY stockyard /usr/local/bin/stockyard
RUN chmod +x /usr/local/bin/stockyard

RUN mkdir -p /data && chown stockyard:stockyard /data

USER stockyard
WORKDIR /data

ENV STOCKYARD_DB_PATH=/data/stockyard.db
ENV STOCKYARD_LOG_FORMAT=json
ENV PORT=4200

EXPOSE 4200

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:4200/health || exit 1

ENTRYPOINT ["stockyard"]
