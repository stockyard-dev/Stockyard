# Reddit Post: r/SideProject

**Title:** I built Stockyard — 20 LLM proxy tools in a single Go binary

**Body:**

I've been building AI apps for the past year and kept running into the same problems: surprise API bills, no caching, no failover when OpenAI goes down, and zero visibility into what my app is actually sending.

So I built Stockyard. It's a proxy that sits between your app and any LLM provider. You change one URL in your app and you get:

- **Cost tracking** with hard spending caps (never get a surprise bill again)
- **Response caching** (stop paying for the same answer twice)
- **Provider failover** (OpenAI down? Traffic goes to Anthropic automatically)
- **Rate limiting** per key, per user, per model
- **Analytics dashboard** with latency percentiles and cost trends
- 15 more tools for prompt management, security, observability, etc.

How it works:
1. Download a single binary (it's Go — no Python, no Docker required)
2. Set your API key and run it
3. Point your app's OPENAI_BASE_URL to localhost:4000
4. Dashboard appears at localhost:4000/ui

Tech stack: Go with no CGO, embedded SQLite, embedded Preact dashboard. Zero external dependencies. The whole thing is a 12MB binary that starts in 50ms.

Works with OpenAI, Anthropic, Gemini, Groq, Ollama, and 12 more providers.

Pricing: 1 product free forever. $9.99/mo per product. $29.99/mo for all 20.

- Site: stockyard.dev
- GitHub: github.com/stockyard-dev/stockyard
- Docs: stockyard.dev/docs

Happy to answer any questions about the architecture or how it works. What LLM infrastructure problems are you running into?
