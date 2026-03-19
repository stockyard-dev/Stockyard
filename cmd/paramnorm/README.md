# ParamNorm

**Normalize parameters across providers.**

ParamNorm calibrates temperature, top_p, and other parameters across models so the same settings produce similar behavior.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/paramnorm

# Your app:   http://localhost:6420/v1/chat/completions
# Dashboard:  http://localhost:6420/ui
```

## What You Get

- Cross-model parameter calibration
- Temperature normalization
- Top_p mapping
- Per-model calibration profiles
- Consistent behavior across providers
- Dashboard with parameter mapping

## Config

```yaml
# paramnorm.yaml
port: 6420
paramnorm:
  calibration:
    temperature:
      gpt-4o: { scale: 1.0 }
      claude-sonnet: { scale: 0.8 }
      gemini-pro: { scale: 1.2 }
```

## Docker

```bash
docker run -p 6420:6420 -e OPENAI_API_KEY=sk-... stockyard/paramnorm
```

## Part of Stockyard

ParamNorm is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
