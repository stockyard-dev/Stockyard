# Stockyard × LobeChat

> Cost caps, caching, and analytics for LobeChat (55K+ ★).

## Setup

### As Model Provider (Recommended)

1. Start Stockyard: `npx @stockyard/stockyard`
2. LobeChat → Settings → Language Model → OpenAI
3. Set API Proxy Address to `http://localhost:4000/v1`
4. Set API Key to `stockyard` (any non-empty string)

All LobeChat conversations now route through Stockyard automatically.

### As Plugin

Install the plugin for in-chat cost queries and analytics:

1. LobeChat → Plugin Store → Install from URL
2. Enter: `https://raw.githubusercontent.com/stockyard/integrations/main/lobechat/plugin.json`

### Docker Sidecar

```bash
OPENAI_API_KEY=sk-... docker compose up
```

See `docker-compose.yml` for LobeChat + Stockyard running together.

## Files

| File | Description |
|------|-------------|
| `plugin.json` | LobeChat plugin manifest with 5 API tools |
| `docker-compose.yml` | One-command LobeChat + Stockyard sidecar |

## License

MIT
