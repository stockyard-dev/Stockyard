# PRODUCTS_SHIPPING.md — Officially Supported Products (v1.0)

These are the products that ship in the v1.0 release. They are fully
implemented, tested, and documented. GoReleaser builds binaries for
all platforms. Docker images are published. npm wrappers work.

## Suite

| Binary     | Product         | Port | Description                         |
|------------|-----------------|------|-------------------------------------|
| stockyard  | Stockyard Suite | 4000 | Unified suite — all products in one |

## Phase 1 — Original Products (Built + Tested)

| Binary       | Product          | Port | Description                            |
|--------------|------------------|------|----------------------------------------|
| costcap      | CostCap          | 4100 | Spend tracking + hard/soft caps        |
| llmcache     | CacheLayer       | 4200 | Exact + semantic response caching      |
| jsonguard    | StructuredShield | 4300 | JSON schema validation + auto-retry    |
| routefall    | FallbackRouter   | 4400 | Provider failover + circuit breaker    |
| rateshield   | RateShield       | 4500 | Rate limiting + token bucket           |
| promptreplay | PromptReplay     | 4600 | Request logging + replay + export      |

## Phase 1 Expansion (Built + Tested)

| Binary       | Product     | Port | Description                       |
|--------------|-------------|------|-----------------------------------|
| keypool      | KeyPool     | 4700 | API key pooling + rotation        |
| promptguard  | PromptGuard | 4710 | PII redaction + injection detect  |
| modelswitch  | ModelSwitch | 4720 | Smart model routing + A/B testing |
| evalgate     | EvalGate    | 4730 | Response quality scoring          |
| usagepulse   | UsagePulse  | 4740 | Per-user/team token metering      |

## Phase 2 Expansion (Built + Tested)

| Binary      | Product     | Port | Description                         |
|-------------|-------------|------|-------------------------------------|
| promptpad   | PromptPad   | 4800 | Versioned prompt templates          |
| tokentrim   | TokenTrim   | 4900 | Context window optimizer            |
| batchqueue  | BatchQueue  | 5000 | Async request queue                 |
| multicall   | MultiCall   | 5100 | Multi-model consensus               |
| streamsnap  | StreamSnap  | 5200 | SSE stream capture + replay         |
| llmtap      | LLMTap      | 5300 | Full API analytics portal           |
| contextpack | ContextPack | 5400 | File/SQLite/URL context injection   |
| retrypilot  | RetryPilot  | 5500 | Intelligent retry + circuit breaker |

## Infrastructure Tools

| Binary    | Description                              |
|-----------|------------------------------------------|
| sy-keygen | License key generation + validation CLI  |
| sy-api    | Stripe checkout + license backend        |
| sy-docs   | Documentation site generator             |

## Total: 22 products + 3 tools = 25 binaries

Everything else in `cmd/` is Phase 3/4 scaffolding. See
[PRODUCTS_BETA.md](PRODUCTS_BETA.md) for experimental products.
