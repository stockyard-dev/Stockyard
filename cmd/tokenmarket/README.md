# TokenMarket

**Reallocate unused API capacity across teams.**

TokenMarket allows teams to request additional token budget from underutilized pools. Auto-rebalance with priority queuing.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tokenmarket

# Your app:   http://localhost:6030/v1/chat/completions
# Dashboard:  http://localhost:6030/ui
```

## What You Get

- Budget pools per team
- Capacity request workflow
- Auto-rebalance unused budget
- Priority queuing
- Usage forecasting
- Dashboard with pool status

## Config

```yaml
# tokenmarket.yaml
port: 6030
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
tokenmarket:
  pools:
    engineering: { budget: 100000, priority: high }
    marketing: { budget: 50000, priority: medium }
    support: { budget: 25000, priority: low }
  rebalance_interval: 1h
```

## Docker

```bash
docker run -p 6030:6030 -e OPENAI_API_KEY=sk-... stockyard/tokenmarket
```

## Part of Stockyard

TokenMarket is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TokenMarket standalone.
