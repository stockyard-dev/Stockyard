# NOTICE — Stockyard Proxy Open Source Boundary

This directory builds the open-source `stockyard-proxy` binary. The source files
in this directory and the `internal/proxy`, `internal/auth`, `internal/config`,
`internal/features`, `internal/mcp`, `internal/provider`, `internal/slog`,
`internal/storage`, `internal/toggle`, and `internal/tracker` packages are
distributed under the Apache 2.0 license (see `LICENSE-APACHE` at the repo root).

## The boundary, honestly

`cmd/stockyard-proxy/main.go` is a 14-line file. It calls
`engine.BootProxy(...)` from the `internal/engine` package. That `engine`
package is currently shared with the BSL-licensed full platform binary
(`cmd/stockyard`). The two binaries are built from the same source tree.

This means **the source tree at this commit cannot be cleanly separated into
"only Apache 2.0 files" and "only BSL files."** If you clone the repository
and run `go build ./cmd/stockyard-proxy/`, the Go toolchain compiles many
files from the BSL platform as part of the dependency graph, even though
the resulting binary only links and runs the proxy code paths.

## What's in the actual binary

Verified by building both targets and comparing:

| Binary                   | Size | Reachable Go packages |
|--------------------------|------|----------------------|
| `cmd/stockyard-proxy`    | 16MB | proxy stack only     |
| `cmd/stockyard` (full)   | 63MB | full platform        |

The 16MB proxy binary contains the core proxy stack (provider routing, model
aliasing, caching, failover, rate limiting, spend tracking, request logging,
~24 middleware modules) and nothing meaningful from the BSL platform.

Go's linker performs dead-code elimination, so functions from BSL packages
that are never reachable from `BootProxy()` get stripped. The 4x size
difference between the two binaries is the BSL platform code that the
linker removed for the proxy build.

A handful of BSL packages (currently 9) leave behind small residual symbols
in the proxy binary — package init() functions and package-level variable
declarations (regex compilation, registry initialization). These are
side-effect-only and total under 100KB. They run at startup but contribute
no functionality to the proxy. They are present because Go cannot eliminate
package init() side effects, even when no other code in the package is
called. The packages currently affected are: `internal/apps/billing` (error
types), `internal/dashboard`, `internal/fault`, `internal/feral`,
`internal/fossil`, `internal/hollow`, `internal/morph`, `internal/site`,
`internal/stampede`. None of their function bodies are linked into the
proxy binary.

## Roadmap to clean separation

The right architectural fix is to extract `BootProxy` and its proxy-only
helpers from `internal/engine` into a new `internal/proxyengine` package
that only imports the proxy stack. This requires moving roughly 1500 lines
of code from `engine.go` and `boot_proxy.go` into the new package, plus
extracting the small middleware files (`auth.go`, `gzip.go`, `recovery.go`).
Tracked at https://github.com/stockyard-dev/stockyard/issues (search
"open-core boundary"). Estimated effort: 1 day of careful refactoring.

The current arrangement is a known limitation, not an attempt to mislead.
We are documenting it explicitly here so you can verify the binary
contents yourself before trusting the Apache 2.0 claim.

## How to verify

```sh
# Build the proxy binary
go build -o sy-proxy ./cmd/stockyard-proxy/

# Compare to the full binary
go build -o sy-full  ./cmd/stockyard/

# Check sizes
ls -lh sy-proxy sy-full

# List the source-level dependency graph (will show many packages)
go list -deps ./cmd/stockyard-proxy/... | grep stockyard-dev/stockyard/internal

# Check what's actually in the compiled binary
strings sy-proxy | grep -c "internal/dashboard"  # init only
strings sy-proxy | grep -c "internal/proxy"      # full proxy code
```

## License summary

- **Compiled `stockyard-proxy` binary**: Apache 2.0
- **Source files in this directory**: Apache 2.0
- **`internal/proxy`, `internal/auth`, `internal/config`, `internal/features`,
  `internal/mcp`, `internal/provider`, `internal/slog`, `internal/storage`,
  `internal/toggle`, `internal/tracker`**: Apache 2.0 (see file headers)
- **`internal/engine` (shared with full platform)**: BSL 1.1, but the
  `BootProxy()` entry point and its reachable code paths are intended for
  Apache 2.0 distribution; this will be enforced via package separation
  in a future commit.
- **Everything else under `internal/`**: BSL 1.1 (see top-level `LICENSE`)
