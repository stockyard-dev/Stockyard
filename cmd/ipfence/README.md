# IPFence

**IP allowlisting for your LLM endpoints.**

IPFence restricts access to your proxy by IP address, CIDR range, or country code. Prevent unauthorized access and bill theft.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/ipfence

# Your app:   http://localhost:5690/v1/chat/completions
# Dashboard:  http://localhost:5690/ui
```

## What You Get

- IP allowlist and denylist
- CIDR range support
- Country-based geofencing
- Automatic GeoIP lookup
- Fail-open or fail-closed modes
- Dashboard with blocked request log

## Config

```yaml
# ipfence.yaml
port: 5690
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
ipfence:
  mode: allowlist  # allowlist | denylist
  allow:
    - 10.0.0.0/8
    - 192.168.0.0/16
    - 203.0.113.50
  deny: []
  block_countries: []  # e.g. [CN, RU]
```

## Docker

```bash
docker run -p 5690:5690 -e OPENAI_API_KEY=sk-... stockyard/ipfence
```

## Part of Stockyard

IPFence is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use IPFence standalone.
