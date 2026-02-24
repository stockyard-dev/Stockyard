# ClusterMode

**Multi-instance Stockyard with shared state.**

ClusterMode runs multiple Stockyard instances with shared state via LiteFS replication or gossip protocol.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/clustermode

# Your app:   http://localhost:6050/v1/chat/completions
# Dashboard:  http://localhost:6050/ui
```

## What You Get

- Multi-instance coordination
- LiteFS SQLite replication
- Leader-follower architecture
- Shared cache across instances
- Health-based leader election
- Dashboard with cluster status

## Config

```yaml
# clustermode.yaml
port: 6050
cluster:
  enabled: true
  mode: litefs  # litefs | gossip
  peers:
    - stockyard-1:6050
    - stockyard-2:6050
  leader_election: true
```

## Docker

```bash
docker run -p 6050:6050 -e OPENAI_API_KEY=sk-... stockyard/clustermode
```

## Part of Stockyard

ClusterMode is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ClusterMode standalone.
