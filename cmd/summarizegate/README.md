# SummarizeGate

**Auto-summarize long contexts to save tokens.**

SummarizeGate scores relevance per context section. Keeps high-relevance verbatim, summarizes low-relevance to save tokens.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/summarizegate

# Your app:   http://localhost:6540/v1/chat/completions
# Dashboard:  http://localhost:6540/ui
```

## What You Get

- Per-section relevance scoring
- Selective summarization
- Token savings tracking
- Configurable relevance threshold
- Preserve high-value content
- Dashboard with savings stats

## Config

```yaml
# summarizegate.yaml
port: 6540
summarizegate:
  relevance_threshold: 0.6
  summarize_model: gpt-4o-mini
  max_summary_ratio: 0.3
```

## Docker

```bash
docker run -p 6540:6540 -e OPENAI_API_KEY=sk-... stockyard/summarizegate
```

## Part of Stockyard

SummarizeGate is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SummarizeGate standalone.
