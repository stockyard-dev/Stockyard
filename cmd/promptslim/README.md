# PromptSlim

**Compress prompts by 40-70% without losing meaning.**

PromptSlim removes filler words, deduplicates instructions, and compresses whitespace in prompts. Configurable aggressiveness levels.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptslim

# Your app:   http://localhost:5830/v1/chat/completions
# Dashboard:  http://localhost:5830/ui
```

## What You Get

- Remove articles and filler words
- Deduplicate repeated instructions
- Compress whitespace
- Configurable aggressiveness
- Before/after token comparison
- Dashboard with savings stats

## Config

```yaml
# promptslim.yaml
port: 5830
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
promptslim:
  aggressiveness: medium  # low | medium | high
  preserve_code_blocks: true
  preserve_urls: true
```

## Docker

```bash
docker run -p 5830:5830 -e OPENAI_API_KEY=sk-... stockyard/promptslim
```

## Part of Stockyard

PromptSlim is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptSlim standalone.
