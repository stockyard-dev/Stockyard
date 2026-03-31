# Contributing to Stockyard

Thanks for wanting to help! Stockyard is a small project and every contribution matters.

## Quick Start

```bash
git clone https://github.com/stockyard-dev/stockyard.git
cd stockyard
make test              # Run all tests
make build             # Build full platform to dist/
make proxy             # Build OSS proxy to dist/
make bench-short       # Run benchmarks (quick)
make pre-push          # Full check: lint + test + bench
```

## Project Structure

```
cmd/stockyard/         # Full platform entry point (BSL 1.1)
cmd/stockyard-proxy/   # OSS proxy entry point (Apache 2.0)
internal/
  engine/              # Boot sequences: Boot() for platform, BootProxy() for OSS
  provider/            # Provider adapters (OSS)
  proxy/               # Middleware chain, toggle registry (OSS)
  auth/                # Users, API keys, provider keys (OSS)
  mcp/                 # MCP server for editor integration (OSS)
  features/            # All middleware modules (OSS core + BSL advanced)
  storage/             # SQLite persistence (OSS)
  config/              # Configuration loading (OSS)
  toggle/              # Runtime module enable/disable (OSS)
  tracker/             # Spend counter + flusher (OSS)
  apps/                # Platform apps: observe, trust, studio, forge, exchange (BSL)
  dashboard/           # Embedded Preact SPA at /ui (BSL)
  platform/            # Product mount system, tier gating (BSL)
  site/                # Marketing site (go:embed from static/)
docs/
  licensing/           # Open-core boundary decision doc
site/                  # Marketing site source (HTML)
.github/
  workflows/           # CI, release, CLA
```

## Making Changes

1. **Fork and branch** from `main`
2. **Write tests** for new features or bug fixes
3. **Run the full suite**: `make pre-push` (lint + test + bench)
4. **Sync site files**: `make site-sync` if you edited `site/`
5. **Open a PR** with a clear description

## Key Make Targets

```bash
make build         # Build full platform binary
make proxy         # Build OSS proxy binary
make test          # All tests with -race
make bench         # Full benchmarks (3 runs)
make bench-short   # Quick benchmarks (1 run)
make lint          # go vet
make check         # Verify both binaries compile
make site-sync     # Copy site/ -> internal/site/static/
make pre-push      # lint + test + bench-short
make docker        # Build Docker image
```

## Code Style

- Go: `gofmt`, short variable names in tight scopes, table-driven tests
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No CGO — everything must compile with `CGO_ENABLED=0`
- No external dependencies unless absolutely necessary (single static binary)
- Tests: use `testing.T`, prefer `httptest` for HTTP, table-driven for multiple cases

## Architecture Rules

These are load-bearing decisions. Don't change them without an RFC:

- **Single binary** — Go + embedded assets, no sidecars
- **SQLite only** — no Postgres, no Redis, no external storage
- **OpenAI-compatible API** — `/v1/chat/completions` is the contract
- **76 middleware modules** — toggleable at runtime via API
- **Open-core boundary** — proxy core (Apache 2.0) vs platform (BSL 1.1), see `docs/licensing/open-core-boundary.md`
- **Preact dashboard** — embedded via `go:embed`, served at `/ui` (BSL binary only)
- **Site files live in two places** — `site/` (source) and `internal/site/static/` (embedded). Always run `make site-sync` after editing site HTML.

## Adding a Middleware Module

1. Implement the `proxy.Middleware` type in the appropriate feature package
2. Add a toggle flag in `internal/proxy/flags.go`
3. Wire into `internal/engine/engine.go` -> `buildMiddlewares()`
4. Add a benchmark case in `internal/proxy/bench_test.go`
5. Update module count in README and site pages

## Adding a Provider

1. Create `internal/provider/yourprovider.go` implementing the `Provider` interface
2. Add request/response translation (convert to/from OpenAI format)
3. Add streaming translation
4. Add tests in `internal/provider/adapter_test.go`
5. Wire into `internal/engine/engine.go` -> `initProviders()`
6. Add pricing data to `internal/provider/pricing.go`
7. Add env var detection to `internal/engine/doctor.go`

## Reporting Issues

Use the [issue templates](https://github.com/stockyard-dev/stockyard/issues/new/choose). Include `stockyard doctor --json` output, OS, and a minimal reproduction. Redact API keys!

## License & CLA

Stockyard uses a dual-license model:
- **Stockyard Proxy** (`cmd/stockyard-proxy/`) is Apache 2.0
- **Stockyard Platform** (`cmd/stockyard/`) is BSL 1.1

Both binaries compile from the same repo. Your contribution may appear in either or both.

Before your first pull request can be merged, you must sign the [Contributor License Agreement](CLA.md). The CLA bot will prompt you automatically — just comment "I have read the CLA Document and I hereby sign the CLA" on your PR. You only need to do this once.

The CLA grants Stockyard the right to include your contribution in both the Apache 2.0 and BSL 1.1 binaries. This is standard practice for open-core projects.
