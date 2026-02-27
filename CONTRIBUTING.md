# Contributing to Stockyard

Thanks for wanting to help! Stockyard is a small project and every contribution matters.

## Quick Start

```bash
git clone https://github.com/stockyard-dev/stockyard.git
cd stockyard
make test              # Run all tests
make build             # Build binary to dist/
make bench-short       # Run benchmarks (quick)
make pre-push          # Full check: lint + test + bench
```

## Project Structure

```
cmd/stockyard/         # Main entry point
internal/
  engine/              # Boot sequence, hooks, config, doctor, OTEL, webhooks, status
  provider/            # OpenAI, Anthropic, Gemini, Groq, Ollama + 12 more adapters
  proxy/               # Middleware chain, toggle registry, benchmarks
  auth/                # Users, API keys, key rotation, provider keys
  apiserver/           # App REST APIs (Observe, Trust, Studio, Forge, Exchange)
  storage/             # SQLite persistence
  license/             # License validation
  dashboard/           # Embedded Preact SPA (/ui)
  site/                # Marketing site (go:embed from static/)
  slog/                # Structured logging
site/                  # Marketing site source (HTML)
  blog/                # Blog posts + RSS feed
  docs/                # 10-page documentation with sidebar nav
  status/              # Live status page
examples/              # Python, Node.js, curl integration samples
vscode-extension/      # VS Code extension (TypeScript)
terraform-provider/    # Terraform provider stub (Go)
mcp/                   # MCP server packages for Claude Desktop / Cursor
configs/               # Example configs (claude_desktop_config.json, etc.)
.github/
  actions/             # GitHub Action: setup-stockyard
  workflows/           # CI: build, test, bench, docker, release
  ISSUE_TEMPLATE/      # Bug report and feature request templates
```

## Making Changes

1. **Fork and branch** from `main`
2. **Write tests** for new features or bug fixes
3. **Run the full suite**: `make pre-push` (lint + test + bench)
4. **Sync site files**: `make site-sync` if you edited `site/`
5. **Open a PR** with a clear description

## Key Make Targets

```bash
make build         # Build binary
make test          # All tests with -race
make bench         # Full benchmarks (3 runs)
make bench-short   # Quick benchmarks (1 run)
make lint          # go vet
make doctor        # Build + run stockyard doctor
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
- **58 middleware modules** — toggleable at runtime via API
- **6 flagship apps** — Proxy, Observe, Trust, Studio, Forge, Exchange
- **Preact dashboard** — embedded via `go:embed`, served at `/ui`
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

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
