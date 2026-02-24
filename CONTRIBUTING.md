# Contributing to Stockyard

Thanks for wanting to help! Stockyard is a small project and every contribution matters.

## Quick Start

```bash
git clone https://github.com/stockyard-dev/stockyard.git
cd stockyard
go test ./...          # Run tests (should all pass)
make build             # Build all 7 binaries to dist/
```

## Project Structure

```
cmd/                   # 7 product entry points (thin main.go files)
internal/
  engine/              # Wires config → middleware → server
  provider/            # OpenAI, Anthropic, Gemini, Groq adapters
  features/            # Middleware: cache, caps, rate limit, failover, validation, logging
  proxy/               # HTTP server, streaming handler
  tracker/             # Token counting, spend tracking
  storage/             # SQLite persistence
  config/              # YAML config parsing
  dashboard/           # Embedded Preact SPA
  api/                 # Management API (/api/*)
test/                  # Integration tests
npm/                   # npx wrapper packages
```

## Making Changes

1. **Fork and branch** from `main`
2. **Write tests** for new features or bug fixes
3. **Run the suite**: `go test ./... -count=1 -race`
4. **Verify all binaries compile**: `make build`
5. **Open a PR** with a clear description

## Code Style

- Go: `gofmt`, short variable names in tight scopes, table-driven tests
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No CGO — everything must compile with `CGO_ENABLED=0`
- No external dependencies unless absolutely necessary (we ship a single static binary)

## Architecture Rules

These are load-bearing decisions. Don't change them without an RFC:

- **Single binary per product** — Go + embedded assets, no sidecars
- **SQLite only** — no Postgres, no Redis, no external storage
- **OpenAI-compatible API** — `/v1/chat/completions` is the contract
- **YAML config** — with `${ENV_VAR}` interpolation
- **Preact dashboard** — embedded via `go:embed`, served at `/ui`

## Adding a Provider

1. Create `internal/provider/yourprovider.go` implementing the `Provider` interface
2. Add request/response translation (convert to/from OpenAI format)
3. Add streaming translation
4. Add mock HTTP server tests in `internal/provider/adapter_test.go`
5. Wire it into `internal/engine/engine.go` → `initProviders()`
6. Add pricing data to `internal/provider/pricing.go`

## Reporting Issues

Use the issue templates. Include your version (`costcap --version`), OS, and a minimal reproduction. Redact API keys!

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
