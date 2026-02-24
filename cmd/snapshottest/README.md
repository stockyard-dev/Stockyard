# SnapshotTest

**Snapshot testing for LLM outputs.**

SnapshotTest records baseline LLM responses and compares future outputs with semantic diffing and configurable thresholds.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/snapshottest

# Your app:   http://localhost:6320/v1/chat/completions
# Dashboard:  http://localhost:6320/ui
```

## What You Get

- Record baseline responses
- Semantic diff (not exact)
- Configurable similarity threshold
- CI-friendly exit codes
- Update snapshots command
- Regression detection

## Config

```yaml
# snapshottest.yaml
port: 6320
snapshottest:
  baseline_dir: ./snapshots/
  threshold: 0.85
  update_command: "snapshottest --update"
```

## Docker

```bash
docker run -p 6320:6320 -e OPENAI_API_KEY=sk-... stockyard/snapshottest
```

## Part of Stockyard

SnapshotTest is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SnapshotTest standalone.
