# Stockyard 18-Channel Launch Playbook
## 27 tasks across 18 channels over 30 days

**Channels:** Twitter, Hacker News, Product Hunt, GitHub, Email, Blog, Reddit, LinkedIn, Discord, Slack, Indie Hackers, Lobsters, Dev.to, Hashnode, Medium, YouTube, HackerNoon, SEO

**Status Key:** `review` = ready for final review → `queued` = scheduled → `posted` = live

---

## Day 1

### 🔴 [TWITTER] Launch Thread (5 tweets)
**Priority:** critical · **Status:** review

```
TWEET 1 (pin this):
Introducing Stockyard — six LLM infrastructure apps in one Go binary.

Proxy. Observe. Trust. Studio. Forge. Exchange.

Zero dependencies. curl -fsSL stockyard.dev/install.sh | sh

You're running in 30 seconds. 🧵

TWEET 2:
The problem: you shipped an app with LLM calls. Now you need cost caps, caching, safety filters, routing, observability, prompt management, and audit trails. That's 6+ separate tools, each with its own Redis/Postgres/Docker setup.

TWEET 3:
Stockyard replaces all of them. One binary. 66 middleware modules. 16 LLM providers. Works with OpenAI, Anthropic, Gemini, Groq, Mistral, and 11 more. Just change your base URL.

TWEET 4:
Try it live — no signup needed. Paste your API key in our playground and route your first request through 66 middleware modules: stockyard.dev/playground

TWEET 5:
Free forever self-hosted. Individual $9.99/mo. Pro $49/mo. Team $149/mo. Enterprise $499/mo. Every tier gets all 16 apps, all 66 modules, all 16 providers. stockyard.dev
```

---

### 🔴 [PRODUCTHUNT] Product Hunt Launch
**Priority:** critical · **Status:** review

```
TAGLINE: Where LLM traffic gets sorted

DESCRIPTION:
Stockyard is a single Go binary that gives you 6 integrated LLM apps: Proxy (66 middleware modules, 16 providers), Observe (traces, costs, anomaly detection), Trust (immutable audit ledger), Studio (prompt management, A/B experiments), Forge (DAG workflow engine), and Exchange (config marketplace).

Zero external dependencies. Runs on embedded SQLite. Self-hosted is free forever.

MAKER COMMENT:
Hey PH! I built Stockyard because I was tired of stitching together 6+ separate tools every time I shipped an LLM-powered app. Proxy routing? One tool. Observability? Another. Cost tracking? Another. Audit trails? Another. Each with its own Redis/Postgres/Docker setup.

Stockyard replaces all of them with one 15MB binary. curl install, 30 seconds to first proxied request. Try the playground — no signup needed: stockyard.dev/playground

GALLERY: Upload 01-hero through 05-forge from marketing/toolkit/cards/product-hunt-gallery/
```

---

### 🔴 [GITHUB] GitHub Release + Discussion
**Priority:** critical · **Status:** review

```
RELEASE: v1.0.0 — Launch Release

Tag: v1.0.0
Title: Stockyard v1.0 — Six apps, one binary, zero dependencies

Body:
The first stable release of Stockyard. A self-hosted LLM proxy and control plane in one Go binary.

Highlights:
- 66 middleware modules, runtime-toggleable via API
- 16 LLM provider integrations
- 62 REST API endpoints
- AES-256-GCM encryption for provider keys at rest
- 400ns per-request overhead (benchmarked)

Apps: Proxy, Observe, Trust, Studio, Forge, Exchange

Get started: curl -fsSL stockyard.dev/install.sh | sh

ALSO: Create a GitHub Discussion (Announcements category) with the same content. Pin it.
```

---

### 🔴 [EMAIL] Launch Email to Waitlist
**Priority:** critical · **Status:** review

```
SUBJECT: Stockyard is live — try it now

Hi {{name}},

Stockyard just launched. Six LLM infrastructure apps in one Go binary. No Redis. No Postgres. No Docker. 30-second install.

What you get:
- Proxy: 66 middleware modules, 16 providers
- Observe: automatic tracing, cost dashboards
- Trust: immutable audit ledger, compliance
- Studio: prompt management, A/B testing
- Forge: workflow DAGs, tool registry
- Exchange: config pack marketplace

Try the playground (no signup): stockyard.dev/playground
Install it: curl -fsSL stockyard.dev/install.sh | sh
Pricing: stockyard.dev/pricing (free tier included)

—Michael, Stockyard

P.S. If you find it useful, a GitHub star helps a lot: github.com/stockyard-dev/stockyard
```

---

### 🔴 [HN] Show HN Submission
**Priority:** critical · **Status:** review

```
TITLE: Show HN: Stockyard – Six LLM apps, one Go binary, zero dependencies
URL: https://stockyard.dev

FIRST COMMENT:
Hey HN, I built Stockyard because I was tired of stitching together separate tools for proxy routing, observability, cost tracking, and compliance every time I shipped an LLM-powered app.

Stockyard is one Go binary that gives you 6 integrated apps:
• Proxy — 66 middleware modules, 16 providers
• Observe — traces, costs, anomaly detection
• Trust — immutable audit ledger, policies
• Studio — prompt management, experiments
• Forge — workflow DAGs, tool registry
• Exchange — config marketplace

The whole thing runs on SQLite — no external databases. Try it live: stockyard.dev/playground. Self-hosted is free forever.
```

---

### 🔴 [BLOG] Why We Built Stockyard
**Priority:** critical · **Status:** draft

```
[BLOG POST — ~2000 words]
# Why We Built Stockyard
Sections: The Problem, One Binary Six Apps, Why Go + SQLite, Architecture, The Vision
```

---

### 🟠 [DISCORD] Discord Communities (5 servers)
**Priority:** high · **Status:** queued

```
Post in:
1. Gophers (#show-and-tell) — Go architecture focus
2. MLOps Community (#tools) — observability + cost tracking angle
3. AI Engineers (#projects) — middleware modules + provider abstraction
4. LangChain (#general) — workflow engine + prompt management
5. Self-Hosted (#new-projects) — zero-dependency angle

TEMPLATE:
Hey! I just launched Stockyard — a single Go binary that replaces 6+ separate LLM tools (proxy, observability, audit, prompts, workflows, config marketplace).

66 middleware modules, 16 providers, embedded SQLite, zero external dependencies. Try the playground: stockyard.dev/playground

Happy to answer any questions!

NOTE: Adapt tone for each server. Gophers wants Go architecture. MLOps wants metrics. Self-Hosted wants deployment simplicity.
```

---

## Day 2

### 🟠 [SLACK] Slack Communities (4 workspaces)
**Priority:** high · **Status:** queued

```
Post in:
1. Gopher Slack (#showcase)
2. DevOps Chat (#tools)
3. AI/ML Slack (#new-tools)
4. Indie Hackers Slack (#launches)

Same template as Discord, adapted per community tone. Keep it short — Slack favors brevity.
```

---

### 🟠 [INDIEHACKERS] Indie Hackers Launch Post
**Priority:** high · **Status:** queued

```
TITLE: I built a self-hosted LLM proxy and control plane in one Go binary — launching today

Sections:
1. The Problem — fragmented tooling for LLM apps
2. What I Built — one binary, six apps, 66 modules
3. The Tech Stack — Go + SQLite, why zero dependencies matters
4. Pricing Model — 4 tiers, free forever self-hosted
5. Launch Results So Far — (fill in after Day 1)
6. What I Learned — solo dev lessons
7. Ask IH — what features would you want?

Link: stockyard.dev
Playground: stockyard.dev/playground
```

---

### 🟠 [REDDIT] r/golang + r/LocalLLaMA + r/selfhosted
**Priority:** high · **Status:** queued

```
Three subreddit posts with tailored messaging for each community. r/golang: architecture focus. r/LocalLLaMA: Ollama support. r/selfhosted: zero-deps self-hosting.
```

---

## Day 3

### 🟠 [LINKEDIN] LLM Middleware Sprawl Article
**Priority:** high · **Status:** queued

```
Your LLM stack doesn't need 12 tools. Full LinkedIn post targeting CTOs and eng managers. Includes hashtags.
```

---

### 🟡 [LOBSTERS] Lobsters Submission
**Priority:** medium · **Status:** queued

```
TITLE: Stockyard: Six LLM apps, one Go binary (stockyard.dev)
URL: https://stockyard.dev
TAGS: go, ai, show

NOTE: Lobsters requires invitation. If you don't have an account, skip or ask someone to submit. Keep the title factual — no hype. Lobsters community values technical substance.
```

---

## Day 5

### 🟠 [YOUTUBE] YouTube: 3-Minute Demo Video
**Priority:** high · **Status:** queued

```
TITLE: Stockyard in 3 Minutes — Complete LLM Platform, One Binary

Script:
0:00 - Hook: 'What if you could replace 6 LLM tools with one binary?'
0:15 - Install: curl command, show terminal
0:30 - First request: proxy a call, show it works
0:45 - Observe: show traces appearing live
1:15 - Trust: show audit ledger with hash chain
1:45 - Studio: show templates, run an experiment
2:15 - Forge: show a workflow DAG executing
2:30 - Exchange: install a pack in one click
2:45 - Pricing: free forever, cloud from $29
3:00 - CTA: stockyard.dev/playground

Use: OBS screen recording + voiceover. Dark theme matches the product.
```

---

### 🟠 [BLOG] Getting Started in 5 Minutes
**Priority:** high · **Status:** queued

```
Tutorial: install → configure → first proxied request → see it in Observe. ~1200 words with code blocks and screenshots.
```

---

## Day 7

### 🟡 [HASHNODE] Hashnode: Why Go + SQLite for LLM Infra
**Priority:** medium · **Status:** queued

```
Cross-post blog post 'Why We Built Stockyard' to Hashnode with additional Go-specific content.

Add sections:
- Why Go over Python for LLM proxy middleware
- SQLite as the embedded database — no ops, no drift
- Benchmark results (400ns per-request overhead)
- Architecture diagram

Tags: Go, SQLite, LLM, AI, Infrastructure
Canonical URL: stockyard.dev/blog/why-i-built-stockyard
```

---

### 🟡 [TWITTER] Module Spotlight Thread
**Priority:** medium · **Status:** queued

```
Thread highlighting 10 of the 66 middleware modules with emoji + one-liner for each.
```

---

## Day 10

### 🟠 [BLOG] Stockyard vs LiteLLM vs Portkey
**Priority:** high · **Status:** queued

```
SEO-targeted comparison. ~2500 words. Honest pros/cons. Target keywords: llm proxy comparison, litellm alternative.
```

---

### 🟡 [SEO] Directory Submissions (10 sites)
**Priority:** medium · **Status:** queued

```
Submit to: awesome-go, awesome-llm-tools, Product Hunt, AlternativeTo, Slant.co, LibHunt, StackShare, Console.dev, Free for Dev, OpenAlternative.co
```

---

### ⚪ [MEDIUM] Medium: LLM Middleware Sprawl
**Priority:** low · **Status:** queued

```
Cross-post the LinkedIn article 'LLM Middleware Sprawl' to Medium.

Publications to target:
- Better Programming
- Towards Data Science
- Level Up Coding

Canonical URL: stockyard.dev/blog/why-i-built-stockyard
Tags: LLM, AI Infrastructure, DevOps, Go, Open Source
```

---

## Day 14

### 🟡 [TWITTER] Week 2 Metrics Update
**Priority:** medium · **Status:** queued

```
Transparency post: stars, installs, cloud signups, playground sessions, dollars proxied. What worked, what didn't.
```

---

## Day 15

### 🟠 [BLOG] 66 Modules Explained
**Priority:** high · **Status:** queued

```
Reference page: one-liner for each of the 66 middleware modules. SEO target: llm proxy middleware.
```

---

### ⚪ [HACKERNOON] HackerNoon: Replacing LiteLLM with One Binary
**Priority:** low · **Status:** queued

```
Cross-post the vs-LiteLLM comparison to HackerNoon.

Angle: 'I Replaced LiteLLM + Langfuse + Redis + Postgres with a Single Go Binary'

Include the comparison table from the PDF whitepaper.
Link to playground for hands-on testing.
Canonical URL: stockyard.dev/vs/litellm
```

---

## Day 16

### ⚪ [DEVTO] Cross-post: Getting Started
**Priority:** low · **Status:** queued

```
Cross-post Getting Started guide to Dev.to. Canonical URL: stockyard.dev/blog/getting-started. Tags: go, opensource, ai, devtools.
```

---

## Day 20

### 🟠 [BLOG] Case Study: Managing LLM Spend
**Priority:** high · **Status:** queued

```
Real stress test data. tenantwall caught budget at $5. Show Observe traces, 429 flow, surprise bill prevention.
```

---

## Day 22

### 🟡 [TWITTER] Observe Demo Video Clip
**Priority:** medium · **Status:** queued

```
30-second screen recording of Observe page with live data. Caption about zero-config cost tracking.
```

---

## Day 25

### 🟠 [BLOG] Stockyard + Vercel AI SDK Guide
**Priority:** high · **Status:** queued

```
Integration guide. ~1500 words with code. SEO: vercel ai sdk proxy, vercel ai sdk cost tracking.
```

---

## Day 30

### 🟠 [TWITTER] 30-Day Recap Thread
**Priority:** high · **Status:** queued

```
Full transparency thread: stars, installs, cloud users, dollars proxied, biggest wins, biggest misses, next 30 days.
```

---


