# CanaryDeploy

**Canary deployments for model changes.**

CanaryDeploy gradually rolls out new models: 5% to 25% to 100%. Auto-promote on quality, auto-rollback on degradation.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/canarydeploy

# Your app:   http://localhost:6620/v1/chat/completions
# Dashboard:  http://localhost:6620/ui
```

## What You Get

- Gradual traffic shifting
- Auto-promote on quality
- Auto-rollback on degradation
- Configurable stages
- Quality comparison
- Dashboard with rollout status

## Config

```yaml
# canarydeploy.yaml
port: 6620
canary:
  old: gpt-4o
  new: gpt-4o-2025-02
  stages: [5, 25, 50, 100]
  promote_threshold: 0.95
  rollback_threshold: 0.80
```

## Docker

```bash
docker run -p 6620:6620 -e OPENAI_API_KEY=sk-... stockyard/canarydeploy
```

## Part of Stockyard

CanaryDeploy is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
