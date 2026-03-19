# EnvSync

**Sync configs and secrets across environments.**

EnvSync manages full environment configs including encrypted secrets with push, promote, diff, and rollback.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/envsync

# Your app:   http://localhost:6480/v1/chat/completions
# Dashboard:  http://localhost:6480/ui
```

## What You Get

- Config + secrets management
- Encrypted secret storage
- Push/promote/diff/rollback
- Pre-promotion validation
- Environment hierarchy
- CLI tool

## Config

```yaml
# envsync.yaml
port: 6480
envsync:
  environments: [dev, staging, production]
  secret_encryption: true
  promotion_chain: dev -> staging -> production
```

## Docker

```bash
docker run -p 6480:6480 -e OPENAI_API_KEY=sk-... stockyard/envsync
```

## Part of Stockyard

EnvSync is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
