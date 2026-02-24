# Stockyard Integrations

> Connect Stockyard to every LLM platform, framework, and tool.

Stockyard integrations let you add cost caps, caching, rate limiting, PII redaction, smart routing, failover, and analytics to any platform that makes LLM API calls — without changing your application code.

## Integration Map

### 🏆 Tier 1 — Self-Hosted AI UIs (P0)

| Platform | Stars | Integration Type | Status |
|----------|-------|-----------------|--------|
| **[Open WebUI](open-webui/)** | 124K+ | Pipeline + Filter + Docker Sidecar | ✅ Ready |
| **[Dify](dify/)** | 100K+ | Model Provider + Workflow Tools | ✅ Ready |
| **[LobeChat](lobechat/)** | 55K+ | Plugin + Docker Sidecar | ✅ Ready |
| **[SillyTavern](sillytavern/)** | 20K+ | Extension (cost display, proxy routing) | ✅ Ready |

### 🔧 Tier 2 — Workflow Automation

| Platform | Integration Type | Status |
|----------|-----------------|--------|
| **[n8n](n8n/)** | Community Node (`n8n-nodes-stockyard`) | ✅ Ready |
| **[Langflow](langflow/)** | Custom Component | ✅ Ready |
| **[Flowise](flowise/)** | Custom Chat Node | ✅ Ready |
| **[Coze](coze/)** | Plugin (ByteDance/Asia market) | ✅ Ready |

### 🤖 Tier 3 — Agent Frameworks

| Framework | Stars | Integration Type | Status |
|-----------|-------|-----------------|--------|
| **[LangChain](langchain/)** | 100K+ | PyPI package (`langchain-stockyard`) | ✅ Ready |
| **[CrewAI](crewai/)** | 25K+ | PyPI package (`crewai-stockyard`) | ✅ Ready |
| **[AutoGen](autogen/)** | 40K+ | PyPI package (`autogen-stockyard`) | ✅ Ready |

### 🖥 Tier 4 — Local LLM Infrastructure

| Platform | Stars | Integration Type | Status |
|----------|-------|-----------------|--------|
| **[Ollama](ollama-sidecar/)** | 100K+ | Docker Sidecar + YAML config | ✅ Ready |
| **vLLM** | 50K+ | Docker Sidecar | ✅ Ready |
| **LocalAI** | 30K+ | Docker Sidecar | ✅ Ready |

### 🐙 Tier 5 — OpenClaw Ecosystem

| Skill | Description | Status |
|-------|-------------|--------|
| **[stockyard-costcap-skill](openclaw/stockyard-costcap-skill/)** | Spend tracking + budget caps | ✅ Ready |
| **[stockyard-cache-skill](openclaw/stockyard-cache-skill/)** | Response caching, save 30-50% | ✅ Ready |
| **[stockyard-analytics-skill](openclaw/stockyard-analytics-skill/)** | Usage analytics + reporting | ✅ Ready |
| **[stockyard-guard-skill](openclaw/stockyard-guard-skill/)** | PII redaction + injection detection | ✅ Ready |
| **[stockyard-router-skill](openclaw/stockyard-router-skill/)** | Smart model routing, save 40-70% | ✅ Ready |
| **[stockyard-full-skill](openclaw/stockyard-full-skill/)** | Complete Stockyard suite | ✅ Ready |

## Universal Pattern

Every integration follows the same pattern:

1. **Stockyard runs as a proxy** on `localhost:4000` (or any port)
2. **Your platform connects** to Stockyard instead of directly to OpenAI/Anthropic
3. **Stockyard forwards** requests to the real LLM provider
4. **Everything is automatic** — cost caps, caching, rate limiting, PII redaction, analytics

```
[Your App] → [Stockyard Proxy :4000] → [OpenAI / Anthropic / Groq / Ollama]
                    ↓
            Cost Caps ✓
            Caching ✓
            Rate Limiting ✓
            PII Redaction ✓
            Smart Routing ✓
            Analytics ✓
```

## Quick Start

```bash
# 1. Start Stockyard
OPENAI_API_KEY=sk-... npx @stockyard/stockyard

# 2. Point your platform at Stockyard
#    Base URL: http://localhost:4000/v1
#    API Key: any-non-empty-string

# 3. Open dashboard
open http://localhost:4000/ui
```

## File Count

| Category | Files |
|----------|-------|
| Open WebUI | 4 (pipeline, filter, docker-compose, README) |
| Dify | 6 (manifest, provider, 3 tools, README) |
| LobeChat | 3 (plugin.json, docker-compose, README) |
| SillyTavern | 3 (extension, manifest, README) |
| n8n | 4 (node, credentials, package.json, README) |
| Langflow | 1 (custom component) |
| Flowise | 1 (custom node) |
| Coze | 1 (plugin manifest) |
| LangChain | 2 (package, setup.py) |
| CrewAI | 2 (package, setup.py) |
| AutoGen | 1 (package) |
| Ollama/vLLM/LocalAI | 4 (3 docker-compose, config) |
| OpenClaw | 18 (6 skills × 3 files each) |
| **Total** | **~50 integration files** |

## License

MIT
