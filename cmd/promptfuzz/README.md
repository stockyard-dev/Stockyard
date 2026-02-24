# PromptFuzz

**Fuzz-test your prompts.**

PromptFuzz generates adversarial, multilingual, and edge case inputs to stress-test prompts. Score with EvalGate.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptfuzz

# Your app:   http://localhost:6260/v1/chat/completions
# Dashboard:  http://localhost:6260/ui
```

## What You Get

- Adversarial input generation
- Multilingual test cases
- Edge case discovery
- EvalGate scoring integration
- Failure report generation
- CLI and API modes

## Config

```yaml
# promptfuzz.yaml
port: 6260
promptfuzz:
  categories: [adversarial, multilingual, edge_case, injection]
  runs_per_category: 50
  target_prompt: "You are a helpful assistant."
```

## Docker

```bash
docker run -p 6260:6260 -e OPENAI_API_KEY=sk-... stockyard/promptfuzz
```

## Part of Stockyard

PromptFuzz is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptFuzz standalone.
