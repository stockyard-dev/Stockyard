# Stockyard — First 5 Tweets

Post these in order, one per day starting when the account is live.

---

## Tweet 1: Introduction (Day 1)

Building Stockyard — LLM proxy tools that just work.

Your app talks to OpenAI/Anthropic/Gemini. Stockyard sits in between and adds cost tracking, caching, rate limiting, and failover.

Single Go binary. No dependencies. Free tier.

stockyard.dev

---

## Tweet 2: Pain point — cost (Day 2)

My LLM API bill hit $847 last month.

I didn't notice until the invoice.

So I built CostCap — set daily/weekly/monthly spend caps per model. Hard limits that actually stop requests before you go broke.

One binary. 10 seconds to set up.

stockyard.dev/products/costcap

---

## Tweet 3: Pain point — reliability (Day 3)

OpenAI went down 3 times last week.

My app went down 0 times.

FallbackRouter: if OpenAI fails, traffic goes to Anthropic. If Anthropic fails, it goes to Gemini. Automatic. Your users never notice.

stockyard.dev/products/routefall

---

## Tweet 4: The "why Go" take (Day 4)

Most LLM proxy tools are Python + Redis + Postgres.

Stockyard is one Go binary. Download and run.

No pip install. No Docker compose. No database to manage. No 200MB runtime.

Just a 12MB binary that starts in 50ms.

github.com/stockyard-dev/stockyard

---

## Tweet 5: Build in public (Day 5)

Stockyard week 1:

- 20 products shipping
- 33,000 lines of Go
- 266 passing tests
- Site live at stockyard.dev
- GitHub repo public

Building LLM infrastructure for indie devs. Free tier on everything.

What LLM pain points are you dealing with?
