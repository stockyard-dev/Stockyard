# Product Hunt Launch #1: CostCap

## Listing Details

**Product name:** CostCap by Stockyard

**Tagline (60 chars):** Stop your LLM API bill from exploding

**Description (260 chars):**
CostCap is a lightweight Go proxy that tracks every dollar you spend on LLM APIs (OpenAI, Anthropic, Gemini, Groq) and enforces hard spending caps. Single binary, embedded dashboard, zero dependencies. Set daily/weekly/monthly limits per model or per key.

**Topics:** Artificial Intelligence, Developer Tools, APIs

**Pricing:** Free, $9.99/mo, $29.99/mo (all products)

**Link:** https://stockyard.dev/products/costcap/

**GitHub:** https://github.com/stockyard-dev/stockyard

---

## Maker Comment (post immediately after launch)

Hey Product Hunt!

I built CostCap because my LLM API bill hit $847 in one month and I didn't notice until the invoice arrived.

The problem: OpenAI, Anthropic, and every other LLM provider will happily charge you unlimited money. There's no built-in way to say "stop at $50/day." If your agent goes into a loop or your traffic spikes, you're paying for it.

CostCap sits between your app and the LLM API. It tracks every request, calculates cost in real-time, and enforces hard limits. Hit your daily cap? Requests get blocked (or downgraded to a cheaper model, your choice).

How it works:
1. Download a single binary (no Python, no Docker required)
2. Set your API key and spending limit
3. Point your app at localhost:4100 instead of api.openai.com
4. That's it. Dashboard at localhost:4100/ui shows live spend.

Technical details:
- Single Go binary, no dependencies
- Embedded SQLite (no external database)
- Works with OpenAI, Anthropic, Gemini, Groq, Ollama, and 12+ more
- OpenAI-compatible API (change one URL in your app)
- Embedded real-time dashboard

CostCap is one of 20 tools in Stockyard, our LLM proxy suite. Free tier is 1,000 requests/day. Paid is $9.99/mo for one product or $29.99/mo for everything.

What's the most you've accidentally spent on LLM APIs? I'd love to hear your horror stories.

---

## Screenshots needed (create before launch)

1. Dashboard showing spend tracking graph
2. Terminal showing CostCap starting up
3. Config file example (simple YAML)
4. Before/after: app code change (one line — just the base URL)

---

## Launch day checklist

- [ ] Submit at 12:01 AM PT (Tuesday or Wednesday)
- [ ] Post maker comment immediately
- [ ] Share on Twitter with link
- [ ] Post on Indie Hackers
- [ ] Reply to every PH comment within 2 hours
- [ ] Share in relevant Discord/Slack communities
- [ ] Post on r/SideProject
