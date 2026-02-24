# PRODUCTS_BETA.md — Experimental / Scaffolded Products

These products exist as `cmd/` entries and have middleware scaffolding,
but are NOT fully tested or documented. They ship as beta binaries
(if included in the release) and are subject to breaking changes.

Do not depend on these in production without testing thoroughly.

## Phase 3 — Priority 1 (Middleware implemented, testing in progress)

ToxicFilter, ComplianceLog, SecretScan, TraceLink, AlertPulse,
ChatMem, MockLLM, TenantWall, IdleKill, IPFence, EmbedCache, AnthroFit

## Phase 3 — Priority 2 (Scaffolded)

AgentGuard, CodeFence, HalluciCheck, TierDrop, DriftWatch,
FeedbackLoop, ABRouter, GuardRail, GeminiShim, LocalSync,
DevProxy, PromptSlim, PromptLint, ApprovalGate, OutputCap,
AgeGate, VoiceBridge, ImageProxy, LangBridge, ContextWindow,
RegionRoute

## Phase 3 — Priority 3 (Scaffolded)

ChainForge, CronLLM, WebhookRelay, BillSync, WhiteLabel,
TrainExport, SynthGen, DiffPrompt, LLMBench, MaskMode,
TokenMarket, LLMSync, ClusterMode, EncryptVault, MirrorTest

## Phase 4 (Planned — cmd/ entry points only)

58 additional products. See the master spec for details.

## What "scaffolded" means

- `cmd/<name>/main.go` exists and compiles
- Config struct defined
- Middleware function has basic logic or stubs
- Dashboard skin may be minimal
- NOT comprehensively tested against real providers
- NOT documented beyond inline comments
- May have incomplete error handling

## GoReleaser behavior

Beta products are built by GoReleaser but packaged separately:
- Release assets: `stockyard-beta-<name>_<os>_<arch>.tar.gz`
- Docker images tagged: `stockyard/<name>:beta`
- Not included in Homebrew formula
- Not included in npm packages

Users who install beta products see a startup warning:

```
WARNING: <product> is in beta. Not recommended for production use.
Report issues at https://github.com/stockyard-dev/stockyard/issues
```
