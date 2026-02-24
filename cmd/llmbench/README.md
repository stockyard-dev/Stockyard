# LLMBench

**Benchmark any model on YOUR workload.**

LLMBench runs your test suite across N models and produces comparison reports on quality, latency, cost, and tokens.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/llmbench

# Your app:   http://localhost:6010/v1/chat/completions
# Dashboard:  http://localhost:6010/ui
```

## What You Get

- Multi-model benchmarking
- Custom test suites
- Quality/latency/cost comparison
- Automated report generation
- Statistical analysis
- CLI and dashboard modes

## Config

```yaml
# llmbench.yaml
port: 6010
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
bench:
  models:
    - gpt-4o
    - gpt-4o-mini
    - claude-sonnet-4-20250514
  test_suite: ./benchmarks/
  runs_per_test: 3
```

## Docker

```bash
docker run -p 6010:6010 -e OPENAI_API_KEY=sk-... stockyard/llmbench
```

## Part of Stockyard

LLMBench is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use LLMBench standalone.
