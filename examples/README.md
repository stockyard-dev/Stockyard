# Stockyard Examples

Code samples showing how to integrate Stockyard with your application.

## Quick Start

```bash
# Start Stockyard
curl -sSL stockyard.dev/install | sh
stockyard

# Your app just changes the base URL
export OPENAI_BASE_URL=http://localhost:4200/v1
```

## Examples

| File | Language | Description |
|------|----------|-------------|
| [python-openai.py](python-openai.py) | Python | Use with the OpenAI Python SDK |
| [node-openai.js](node-openai.js) | Node.js | Use with the OpenAI Node SDK |
| [curl-basics.sh](curl-basics.sh) | Shell | Raw curl examples for all endpoints |
| [webhook-setup.sh](webhook-setup.sh) | Shell | Set up Slack/webhook alerting |
| [docker-compose.yml](docker-compose.yml) | YAML | Production Docker deployment |

## Integration Patterns

**Drop-in proxy:** Change `base_url` / `OPENAI_BASE_URL` in your existing code.
No SDK changes, no wrapper libraries, no code modifications beyond the URL.

**Multi-provider failover:** Stockyard automatically fails over between providers.
Set multiple API keys and the fallback router handles the rest.

**CI/CD:** Use the GitHub Action to run Stockyard in your test pipelines.
Every test request gets cost tracking, caching, and audit trails.
