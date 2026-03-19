# Stockyard Social Media Setup Kit
## Every field, every bio, every first post — copy and paste

---

## 1. TWITTER / X (@stockaboryard)

### Profile Setup
- **Name:** Stockyard
- **Handle:** @stockyarddev (or @stockyard_ if taken)
- **Bio (160 chars):**
  ```
  Open-source LLM platform. Six apps, one Go binary, zero dependencies. Proxy · Observe · Trust · Studio · Forge · Exchange. Free forever self-hosted.
  ```
- **Location:** Open Source
- **Website:** https://stockyard.dev
- **Profile image:** `profile-400.png`
- **Banner:** `twitter-banner.png`
- **Category:** Technology
- **Pinned tweet:** The launch thread (below)

### Day 1 — Launch Thread (pin this)

**Tweet 1:**
```
Introducing Stockyard — six LLM infrastructure apps in one Go binary.

Proxy. Observe. Trust. Studio. Forge. Exchange.

Zero dependencies. curl -sSL stockyard.dev/install.sh | sh

You're running in 30 seconds. 🧵
```

**Tweet 2:**
```
The problem: you shipped an app with LLM calls. Now you need cost caps, caching, safety filters, routing, observability, prompt management, and audit trails.

That's 6+ separate tools, each with its own Redis/Postgres/Docker setup.
```

**Tweet 3:**
```
Stockyard replaces all of them. One binary. 58 middleware modules. 16 LLM providers. Works with OpenAI, Anthropic, Gemini, Groq, Mistral, and 11 more.

Just change your base URL.
```

**Tweet 4:**
```
Try it live — no signup needed. Paste your API key in our playground and route your first request through 58 middleware modules:

stockyard.dev/playground
```

**Tweet 5:**
```
Free forever self-hosted.

Individual $9.99/mo
Pro $49/mo
Team $149/mo
Enterprise $499/mo

Every tier gets all 6 apps, all 58 modules, all 16 providers.

stockyard.dev
```

---

## 2. HACKER NEWS

### Show HN Submission

**Title:**
```
Show HN: Stockyard – Six LLM apps, one Go binary, zero dependencies
```

**URL:**
```
https://stockyard.dev
```

**First comment (post immediately after submitting):**
```
Hey HN, I built Stockyard because I was tired of stitching together separate tools for proxy routing, observability, cost tracking, and compliance every time I shipped an LLM-powered app.

Stockyard is one Go binary that gives you 6 integrated apps:

• Proxy — 58 middleware modules, 16 providers
• Observe — traces, costs, anomaly detection
• Trust — immutable audit ledger, policies
• Studio — prompt management, experiments
• Forge — workflow DAGs, tool registry
• Exchange — config marketplace

The whole thing runs on SQLite — no external databases. AES-256-GCM encryption for provider keys at rest. 400ns per-request overhead across the full 58-module chain (benchmarked on Xeon Platinum).

Try it live: stockyard.dev/playground — paste your OpenAI key and route a request. Self-hosted is free forever.

Stack: Go 1.22, embedded SQLite, Preact dashboard. ~15MB binary. Ships as `curl | sh` or Docker.

Happy to answer any architecture questions. I'm particularly proud of the hash-chained audit ledger in Trust and the DAG workflow engine in Forge.
```

---

## 3. PRODUCT HUNT

### Profile
- **Name:** Stockyard
- **Profile image:** `ph-profile.png`

### Launch
- **Tagline (60 chars max):**
  ```
  Where LLM traffic gets sorted
  ```

- **Description:**
  ```
  Stockyard is a single Go binary that gives you 6 integrated LLM apps: Proxy (58 middleware modules, 16 providers), Observe (traces, costs, anomaly detection), Trust (immutable audit ledger), Studio (prompt management, A/B experiments), Forge (DAG workflow engine), and Exchange (config marketplace).

  Zero external dependencies. Runs on embedded SQLite. Self-hosted is free forever.
  ```

- **Maker comment:**
  ```
  Hey Product Hunt! 👋

  I built Stockyard because I was tired of stitching together 6+ separate tools every time I shipped an LLM-powered app. Proxy routing? One tool. Observability? Another. Cost tracking? Another. Audit trails? Another. Each with its own Redis/Postgres/Docker setup.

  Stockyard replaces all of them with one 15MB binary. curl install, 30 seconds to first proxied request.

  The whole thing runs on Go + embedded SQLite. No external databases, no Docker dependencies, no config files required. AES-256-GCM encryption for all provider keys at rest.

  Try the playground — no signup needed: stockyard.dev/playground

  I'm the solo developer, happy to answer anything about the architecture, business model, or roadmap!
  ```

- **Topics:** Developer Tools, Open Source, Artificial Intelligence, SaaS
- **Gallery images:** Upload 01-05 from `marketing/toolkit/cards/product-hunt-gallery/`
- **Pricing:** Freemium
- **Website:** https://stockyard.dev

---

## 4. GITHUB

### Repository Settings
- **Description:**
  ```
  The complete LLM infrastructure platform. Six apps, one binary, zero dependencies. Proxy · Observe · Trust · Studio · Forge · Exchange.
  ```
- **Website:** https://stockyard.dev
- **Topics:** `llm` `proxy` `go` `sqlite` `ai` `middleware` `observability` `devtools` `open-source` `infrastructure`
- **Social preview:** Upload `marketing/toolkit/cards/github-social.png`

### Release v1.0.0
- **Tag:** v1.0.0
- **Title:** `Stockyard v1.0 — Six apps, one binary, zero dependencies`
- **Body:**
  ```
  The first stable release of Stockyard. A complete LLM infrastructure platform in a single Go binary.

  ### Highlights
  - 58 middleware modules, runtime-toggleable via API
  - 16 LLM provider integrations
  - 62 REST API endpoints
  - AES-256-GCM encryption for provider keys at rest
  - 400ns per-request overhead (benchmarked)

  ### The 6 Apps
  - **Proxy** — Gateway layer with 58 middleware modules
  - **Observe** — Tracing, cost dashboards, anomaly detection
  - **Trust** — Hash-chained audit ledger, policies, compliance
  - **Studio** — Prompt templates, A/B experiments, benchmarks
  - **Forge** — DAG workflow engine, tool registry
  - **Exchange** — Config pack marketplace

  ### Get Started
  ```bash
  curl -sSL stockyard.dev/install.sh | sh
  stockyard serve
  ```

  ### Links
  - Website: https://stockyard.dev
  - Playground: https://stockyard.dev/playground
  - Docs: https://stockyard.dev/docs
  - Pricing: https://stockyard.dev/pricing
  ```

### Pinned Discussion (Announcements)
- **Title:** `🚀 Stockyard v1.0 is live`
- **Body:** Same as release body above, plus:
  ```
  We'd love your feedback! Try the playground, star the repo, and let us know what you think in this thread.

  Roadmap and feature requests: [link to Issues]
  ```

---

## 5. LINKEDIN (Company Page)

### Page Setup
- **Company name:** Stockyard
- **Tagline (120 chars):**
  ```
  Open-source LLM infrastructure platform. Six apps, one Go binary, zero dependencies.
  ```
- **Industry:** Software Development
- **Company size:** 1 employee
- **Type:** Privately Held
- **Website:** https://stockyard.dev
- **Logo:** `profile-800.png`
- **Cover:** `linkedin-banner.png`

### About (2000 chars max):
```
Stockyard is the complete LLM infrastructure platform in a single Go binary. Instead of stitching together separate tools for proxy routing, observability, cost tracking, audit trails, prompt management, and workflow orchestration, Stockyard gives you six integrated apps:

Proxy — 58 middleware modules with runtime toggles. Route through OpenAI, Anthropic, Gemini, Groq, Mistral, and 11 more providers. Caching, rate limiting, cost caps, safety filters, failover — all in the request chain.

Observe — Automatic tracing, cost dashboards, anomaly detection, and real-time alerts for every LLM request.

Trust — Append-only audit ledger with hash-chain integrity. Evidence packs, policy enforcement, and compliance logging.

Studio — Versioned prompt templates, A/B experiments, model benchmarks, and snapshot comparison.

Forge — DAG workflow engine. Chain LLM calls, transforms, and tool calls with dependency ordering.

Exchange — Config pack marketplace. Install providers, modules, routes, and workflows in one click.

Zero external dependencies. Embedded SQLite. AES-256-GCM encryption for provider keys at rest. Self-hosted is free forever.

stockyard.dev
```

### Day 1 — Launch Post:
```
Your LLM stack doesn't need 12 separate tools.

I just shipped Stockyard — six LLM infrastructure apps in one Go binary.

→ 58 middleware modules, every one toggleable at runtime
→ 16 LLM providers, unified API
→ 400ns per-request overhead (benchmarked)
→ AES-256-GCM encryption for all provider keys
→ Zero external dependencies — no Redis, no Postgres, no Docker

One curl command, 30 seconds to your first proxied request.

Free forever self-hosted. Paid tiers from $9.99/mo.

Try the playground (no signup): stockyard.dev/playground

#DevTools #LLM #OpenSource #GoLang #AI #Infrastructure
```

---

## 6. REDDIT

### Day 2 Posts (3 subreddits, tailored)

**r/golang:**
```
Title: Show r/golang: Built a complete LLM platform in Go — single binary, embedded SQLite, 58 middleware modules

Body:
I've been working on Stockyard, an LLM infrastructure platform that ships as a single Go binary with zero external dependencies.

Architecture highlights that might interest this community:

- 58 middleware modules in a chain, each implementing a simple interface — composable, runtime-toggleable via API
- Embedded SQLite (mattn/go-sqlite3) — no external DB
- 62 REST endpoints served on a single port with Go 1.22 routing
- Benchmarked at 400ns per-request overhead for the full 58-module chain on Xeon Platinum
- AES-256-GCM encryption for provider keys at rest
- ~15MB binary, CGO_ENABLED=0 builds

The middleware chain pattern turned out really clean — each module gets the request context, can modify it, and calls next(). Toggle any module on/off at runtime via PUT /api/proxy/modules/{name}.

Would love feedback on the architecture. Source: github.com/stockyard-dev/stockyard
Live playground: stockyard.dev/playground
```

**r/selfhosted:**
```
Title: Stockyard — self-hosted LLM platform, single binary, zero dependencies, free forever

Body:
Built an LLM infrastructure platform designed for self-hosting:

- Single Go binary (~15MB), zero external dependencies
- Embedded SQLite — no Postgres, no Redis, no Docker required
- curl -sSL stockyard.dev/install.sh | sh → running in 30 seconds
- 6 integrated apps: proxy, observability, audit trails, prompt management, workflows, config marketplace
- All 58 middleware modules and 16 provider integrations included in the free tier
- AES-256-GCM encryption for provider API keys at rest
- Works with Ollama for fully local inference

Self-hosted community tier is free forever with no artificial limits on features — just a 10K request/month cap.

stockyard.dev | GitHub: github.com/stockyard-dev/stockyard
```

**r/LocalLLaMA:**
```
Title: Stockyard — open-source LLM proxy that works with Ollama, VLLM, and 14 other providers

Body:
I built Stockyard as a unified gateway for LLM traffic. It sits between your app and your model providers — whether that's Ollama running locally, VLLM on your GPU server, or cloud providers like OpenAI/Anthropic.

What it gives you:

- OpenAI-compatible API — point OPENAI_BASE_URL at Stockyard and your existing code works
- 58 middleware modules: caching (saves re-running inference), rate limiting, cost tracking, safety filters, prompt management
- Works with Ollama, VLLM, OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, and 8 more
- Single binary, embedded SQLite, zero dependencies
- Failover routing — if your local Ollama is down, automatically route to a cloud provider

Try it: stockyard.dev/playground | GitHub: github.com/stockyard-dev/stockyard
```

---

## 7. DISCORD (5 servers)

### Template (adapt per server):
```
Hey! Just launched Stockyard — a single Go binary that replaces 6+ separate LLM tools.

58 middleware modules · 16 providers · embedded SQLite · zero external dependencies

What it does: sits between your app and LLM providers. Gives you caching, rate limiting, cost tracking, safety filters, audit trails, prompt management, and workflow orchestration — all in one binary.

Try the playground (no signup): stockyard.dev/playground
GitHub: github.com/stockyard-dev/stockyard

Happy to answer questions!
```

**Server-specific adjustments:**
- **Gophers (#show-and-tell):** Lead with Go architecture, mention the middleware chain pattern, benchmark numbers
- **MLOps Community (#tools):** Lead with observability — traces, cost dashboards, anomaly detection
- **AI Engineers (#projects):** Lead with provider abstraction — 16 providers, OpenAI-compatible API
- **LangChain (#general):** Lead with workflow engine and prompt management
- **Self-Hosted (#new-projects):** Lead with zero dependencies, curl install, free forever

---

## 8. SLACK (4 workspaces)

Same template as Discord, shortened:
```
Launched Stockyard — complete LLM platform in a single Go binary. 58 middleware modules, 16 providers, zero dependencies.

Proxy + observability + audit trails + prompt management + workflow engine + config marketplace.

Free self-hosted: stockyard.dev/playground
```

**Workspaces:** Gopher Slack (#showcase), DevOps Chat (#tools), AI/ML Slack (#new-tools), Indie Hackers (#launches)

---

## 9. INDIE HACKERS

### Post:
```
Title: I built a complete LLM platform in a single Go binary — launching today

I'm Michael, solo founder of Stockyard. Here's the story.

## The Problem
Every time I shipped an app with LLM calls, I needed cost caps, caching, safety filters, routing, observability, prompt management, and audit trails. That's 6+ separate tools, each with Redis/Postgres/Docker dependencies.

## What I Built
One Go binary. Six integrated apps. Zero external dependencies.

- Proxy: 58 middleware modules, 16 providers
- Observe: traces, cost dashboards, anomaly detection
- Trust: immutable audit ledger, compliance
- Studio: prompt templates, A/B experiments
- Forge: DAG workflow engine
- Exchange: config pack marketplace

## Tech Stack
Go + embedded SQLite. ~15MB binary. AES-256-GCM encryption. 400ns per-request overhead.

## Pricing
Free forever self-hosted. Individual $9.99/mo. Pro $49/mo. Team $149/mo. Enterprise $499/mo. Every tier gets everything.

## Try It
Playground (no signup): stockyard.dev/playground
GitHub: github.com/stockyard-dev/stockyard

What features would you want in something like this?
```

---

## 10. DEV.TO

### Profile
- **Name:** Stockyard
- **Bio:** Open-source LLM infrastructure platform. Six apps, one Go binary.
- **Website:** https://stockyard.dev

### First Post (cross-post Getting Started guide):
- **Title:** `Getting Started with Stockyard in 5 Minutes`
- **Tags:** `go`, `opensource`, `ai`, `devtools`
- **Canonical URL:** `https://stockyard.dev/guide/`
- Reformat the guide page content for Dev.to markdown

---

## 11. LOBSTERS

```
Title: Stockyard: Six LLM apps, one Go binary (stockyard.dev)
URL: https://stockyard.dev
Tags: go, ai, show
```
Note: Requires invitation to post. Keep title factual.

---

## 12. YOUTUBE

### Channel Setup
- **Name:** Stockyard
- **Handle:** @stockyarddev
- **Description:**
  ```
  Stockyard is the complete LLM infrastructure platform. Six apps, one binary, zero dependencies. Tutorials, demos, and deep dives into LLM infrastructure.

  Website: https://stockyard.dev
  GitHub: https://github.com/stockyard-dev/stockyard
  Docs: https://stockyard.dev/docs
  ```
- **Profile image:** `youtube-profile.png`
- **Banner:** `youtube-banner.png`
- **Links:** stockyard.dev, github.com/stockyard-dev/stockyard

---

## 13. HASHNODE

### Profile
- **Name:** Stockyard
- **Bio:** Open-source LLM infrastructure. Go + SQLite.
- **Website:** https://stockyard.dev
- **Blog subdomain:** stockyard.hashnode.dev

---

## 14. MEDIUM

### Publication
- **Name:** Stockyard
- **Description:** Building the complete LLM infrastructure platform in Go.
- **Tags:** LLM, AI Infrastructure, DevOps, Go, Open Source

---

## 15. SEO DIRECTORY SUBMISSIONS (Day 10)

Submit to all 10 with this info:

| Field | Value |
|-------|-------|
| Name | Stockyard |
| URL | https://stockyard.dev |
| GitHub | https://github.com/stockyard-dev/stockyard |
| Description | Open-source LLM infrastructure platform. Six apps (proxy, observability, audit, prompts, workflows, marketplace) in a single Go binary with zero dependencies. |
| Category | Developer Tools / AI / Infrastructure |
| License | MIT |
| Language | Go |

**Directories:**
1. awesome-go (PR to github.com/avelino/awesome-go)
2. Product Hunt (already done)
3. AlternativeTo (alternative to LiteLLM, Helicone, Portkey)
4. StackShare
5. Console.dev
6. LibHunt
7. Slant.co
8. OpenAlternative.co
9. Free for Dev (github.com/ripienaar/free-for-dev)
10. awesome-llm-tools (search GitHub for the most popular list)

---

## IMAGE FILES INCLUDED

| File | Size | Use |
|------|------|-----|
| `profile-400.png` | 400×400 | Twitter/X |
| `profile-800.png` | 800×800 | LinkedIn, YouTube |
| `profile-1024.png` | 1024×1024 | Generic / high-res |
| `github-profile.png` | 500×500 | GitHub org |
| `discord-profile.png` | 512×512 | Discord |
| `ph-profile.png` | 240×240 | Product Hunt |
| `twitter-banner.png` | 1500×500 | Twitter/X header |
| `linkedin-banner.png` | 1584×396 | LinkedIn cover |
| `youtube-banner.png` | 2560×1440 | YouTube channel art |

Also use from existing repo:
| File | Use |
|------|-----|
| `marketing/toolkit/cards/og-card.png` | Open Graph / link previews |
| `marketing/toolkit/cards/github-social.png` | GitHub repo social preview |
| `marketing/toolkit/cards/product-hunt-gallery/*` | PH launch gallery |

---

## LAUNCH DAY CHECKLIST

### Before you start (30 min):
- [ ] Open all platform tabs in browser
- [ ] Have profile images ready to upload
- [ ] Copy this doc to a notes app for quick paste

### Execute in this order:
1. [ ] **Product Hunt** — submit first (scheduled launch if possible)
2. [ ] **GitHub** — create release v1.0.0, pin discussion
3. [ ] **Twitter/X** — post thread, pin tweet 1
4. [ ] **Hacker News** — submit Show HN, post first comment immediately
5. [ ] **LinkedIn** — publish launch post
6. [ ] **Email** — send launch email to captured leads
7. [ ] **Discord** — post in 5 servers
8. [ ] **Blog** — publish "Why We Built Stockyard" (if not already live)

### Day 2:
9. [ ] **Reddit** — post in r/golang, r/selfhosted, r/LocalLLaMA
10. [ ] **Slack** — post in 4 workspaces
11. [ ] **Indie Hackers** — publish launch post

### Day 3:
12. [ ] **Lobsters** — submit (if you have access)

### Day 5-10:
13. [ ] **YouTube** — record and upload 3-min demo
14. [ ] **Hashnode** — cross-post blog
15. [ ] **Dev.to** — cross-post Getting Started
16. [ ] **SEO** — submit to 10 directories
17. [ ] **Medium** — cross-post article

### Day 15-30:
18. [ ] **HackerNoon** — publish vs-LiteLLM comparison
