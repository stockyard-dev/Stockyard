# SlotFill

**Form-filling conversation engine.**

SlotFill provides declarative form-through-conversation. Define slots with types and validation, track fill state, auto-reprompt.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/slotfill

# Your app:   http://localhost:6210/v1/chat/completions
# Dashboard:  http://localhost:6210/ui
```

## What You Get

- Declarative slot definitions
- Type validation per slot
- Auto-reprompt for missing slots
- Completion funnel tracking
- Custom validation functions
- Dashboard with fill rates

## Config

```yaml
# slotfill.yaml
port: 6210
slotfill:
  forms:
    booking:
      slots:
        - { name: date, type: date, required: true }
        - { name: guests, type: integer, min: 1, max: 20 }
        - { name: notes, type: string, required: false }
```

## Docker

```bash
docker run -p 6210:6210 -e OPENAI_API_KEY=sk-... stockyard/slotfill
```

## Part of Stockyard

SlotFill is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
