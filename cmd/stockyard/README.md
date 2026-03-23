# Stockyard

**Self-hosted LLM proxy and control plane. One Go binary.**

58 middleware modules, 16 providers, embedded SQLite. Zero external dependencies.

## Quickstart

```bash
curl -fsSL https://stockyard.dev/install.sh | sh
stockyard

# Proxy:   http://localhost:4200/v1
# Console: http://localhost:4200/ui
```

## Build from Source

```bash
CGO_ENABLED=0 go build -o stockyard ./cmd/stockyard/
./stockyard
```

## What You Get

- 58 middleware modules (rate limiting, caching, cost caps, safety, failover, observability)
- 16 LLM provider integrations
- 6 integrated apps: Proxy, Observe, Trust, Studio, Forge, Exchange
- OpenAI-compatible API endpoint
- Embedded SQLite with WAL mode
- AES-256-GCM encryption for provider keys at rest
- ~20MB static binary

## More

- Website: https://stockyard.dev
- Docs: https://stockyard.dev/docs
- Playground: https://stockyard.dev/playground

MIT licensed.
