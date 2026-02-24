# RELEASE_CHECKLIST.md — Stockyard Release Process

## 1. Bootstrap (first time or after dep changes)

```bash
./bootstrap.sh
```

Installs SQLite driver, runs `go mod tidy`, verifies pure-Go build.

## 2. Run full test suite

```bash
make test
```

All tests must pass. Do not ship with failures.

## 3. Verify shipping binaries compile

```bash
make check
```

Compiles every binary in PRODUCTS_SHIPPING.md without output.

## 4. Build all binaries

```bash
make build
```

Produces `dist/<binary>` for every shipping product.

## 5. Export the website

```bash
./export-site.sh
```

Produces `dist/` with docs (133 pages) + products (126 pages) + assets.

## 6. Check for broken links

```bash
./check-links.sh
```

Must exit 0. Any broken link is a release blocker.

## 7. GoReleaser dry run

```bash
make snapshot
```

## 8. Tag and release

```bash
git tag v1.0.0
git push origin v1.0.0
make release
```

## 9. Post-release verification

- [ ] GitHub release page shows all binaries
- [ ] `brew install stockyard-dev/tap/stockyard` works
- [ ] `npx stockyard --version` works
- [ ] `docker run stockyard/stockyard --version` works
- [ ] Landing pages + docs deploy without 404s

## Known issues

**SQLite driver**: `modernc.org/sqlite` must be in `go.mod`. If you see
`"unknown driver sqlite"` at runtime, run `./bootstrap.sh`.

**Module download**: After cloning, run `go mod download` before building.
The `bootstrap.sh` script handles this automatically.
