# Stockyard × Ollama / vLLM / LocalAI

> Auth, rate limiting, cost tracking, and analytics for local LLMs.

None of these exist natively in Ollama, vLLM, or LocalAI. Stockyard adds them as a zero-config Docker sidecar.

## Ollama

```bash
docker compose up  # Uses docker-compose.yml
```

Open WebUI or any client connects to `http://localhost:4000/v1` instead of `http://localhost:11434`.

## vLLM

```bash
docker compose -f docker-compose.vllm.yml up
```

## What You Get

| Feature | Ollama Native | With Stockyard |
|---------|:---:|:---:|
| Auth/API keys | ❌ | ✅ |
| Rate limiting | ❌ | ✅ |
| Request caching | ❌ | ✅ |
| Cost tracking | ❌ | ✅ |
| Analytics dashboard | ❌ | ✅ |
| Request logging | ❌ | ✅ |
| PII redaction | ❌ | ✅ |

## License

MIT
