# PersonaSwitch

**Hot-swap AI personalities.**

PersonaSwitch manages named personality profiles with prompt, temperature, and format rules. Route by header, key, or user segment.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/personaswitch

# Your app:   http://localhost:6560/v1/chat/completions
# Dashboard:  http://localhost:6560/ui
```

## What You Get

- Named persona profiles
- Per-persona system prompts
- Temperature and format per persona
- Route by header or key
- A/B testing personas
- Dashboard with persona usage

## Config

```yaml
# personaswitch.yaml
port: 6560
personas:
  formal:
    system_prompt: "You are a professional business assistant."
    temperature: 0.3
  casual:
    system_prompt: "You're a friendly, casual helper. Keep it chill."
    temperature: 0.8
default: formal
route_by: header  # X-Persona header
```

## Docker

```bash
docker run -p 6560:6560 -e OPENAI_API_KEY=sk-... stockyard/personaswitch
```

## Part of Stockyard

PersonaSwitch is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PersonaSwitch standalone.
