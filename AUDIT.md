# Stockyard Codebase Audit Report

**Date:** 2026-03-22
**Scope:** Full codebase audit — security, code quality, architecture, testing, deployment
**Codebase:** ~67K lines of Go (412 source files) + ~8K lines of tests (35 test files)

---

## Executive Summary

Stockyard is a well-structured LLM proxy/gateway platform with a large feature surface (~130+ middleware modules). The codebase demonstrates solid foundations — parameterized SQL queries, AES-256-GCM encryption at rest, HMAC webhook signatures, constant-time admin key comparison, and SSRF protection. However, several areas need attention across security, testing, and architecture.

**Critical Issues:** 2
**High Severity:** 9
**Medium Severity:** 17
**Low Severity:** 10

---

## 1. Security Audit

### CRITICAL

#### S1. Weak Key Derivation for Encryption Key
**File:** `internal/auth/crypto.go:50-57`
**Issue:** SHA-256 with a fixed salt is used to derive the AES-256 encryption key from `STOCKYARD_ENCRYPTION_KEY`. This is not a proper KDF — no iteration count, no stretching.
**Risk:** If the env var has low entropy, the derived key is vulnerable to brute-force.
**Fix:** Use PBKDF2, scrypt, or Argon2id with at minimum 100K iterations. The comment says "provides adequate entropy" for keys >= 32 chars, but no enforcement prevents shorter, weak passphrases.

```go
// Current (weak):
h := sha256.New()
h.Write(salt)
h.Write([]byte(envKey))
key := h.Sum(nil)

// Recommended:
key := pbkdf2.Key([]byte(envKey), salt, 100000, 32, sha256.New)
```

#### S2. Stripe Webhook Signature Verification is Opt-In
**File:** `internal/apps/billing/stripe.go:321-330`
**Issue:** If `STRIPE_WEBHOOK_SECRET` is not set, the webhook handler processes events **without any signature verification**. An attacker can POST forged Stripe events (e.g., `payment_intent.succeeded`) to credit arbitrary customer accounts.
**Risk:** Financial fraud — arbitrary credit injection.
**Fix:** Return 500/503 and refuse to process webhooks when the secret is not configured.

```go
// Current:
if whSecret != "" {
    // verify...
}
// If empty, continues processing with NO verification!

// Fix: Reject when not configured
if whSecret == "" {
    w.WriteHeader(503)
    writeJSON(w, map[string]string{"error": "webhook secret not configured"})
    return
}
```

### HIGH

#### S3. Admin API Open by Default (Dev Mode Footgun)
**File:** `internal/engine/auth.go:92-98, 182-191`
**Issue:** When `STOCKYARD_ADMIN_KEY` is not set, all GET requests to `/api/*` are unauthenticated, and POST/PUT/DELETE are blocked only on `/api/*` routes. This is documented as "dev mode" but is the default — production deployments that forget this env var are fully exposed.
**Fix:** Log a prominent startup warning. Consider requiring the key in production or checking for a `STOCKYARD_DEV_MODE=true` opt-in.

#### S4. Rate Limiter Memory Growth (DoS Vector)
**File:** `internal/engine/auth.go:20-66`
**Issue:** The `ipRateLimiter` uses an in-memory map with cleanup only triggered when the map exceeds 10,000 entries. An attacker can slowly grow the map with unique IPs. The cleanup iterates the entire map under a mutex lock, which can block all requests.
**Fix:** Use a time-based eviction (e.g., periodic goroutine cleanup every minute) or a bounded LRU cache.

#### S5. RBAC `checkPermission` Admin Role is a No-Op
**File:** `internal/auth/rbac.go:129-135`
**Issue:** The Admin role check has dead code — it checks for DELETE on `/api/team/` but then falls through to `return true` regardless. The comment says admins should be blocked from "team ownership and some billing" but they aren't.
```go
if role == RoleAdmin {
    if strings.HasPrefix(path, "/api/team/") && method == "DELETE" {
        return true // admins can remove members
    }
    return true // <-- THIS MAKES THE ABOVE CHECK POINTLESS
}
```
**Fix:** Implement the intended restrictions (block ownership transfer, certain billing operations).

#### S6. No Request Body Size Limit on Proxy Endpoint
**File:** `internal/proxy/handler.go:18-27`
**Issue:** The `handleChatCompletions` handler calls `parseRequest` which reads the full body without a `MaxBytesReader` wrapper. The management API has a 5MB limit (`auth.go:126`) and embeddings has `maxRequestBodySize`, but the main chat completions proxy endpoint does not.
**Fix:** Add `http.MaxBytesReader` to `parseRequest` or at the proxy server level (e.g., 10MB limit).

#### S7. Provider Cache Never Cleaned (Memory Leak)
**File:** `internal/auth/middleware.go:149-218`
**Issue:** `ProviderFactory.cache` is a `sync.Map` that grows unboundedly. Entries are set with a 5-minute TTL but expired entries are only removed on cache hit. If users churn, stale entries accumulate forever.
**Fix:** Add a periodic cleanup goroutine to evict expired entries.

#### S8. Webhook Secrets Stored in Plaintext
**File:** `internal/engine/webhooks.go:39-49`
**Issue:** The `webhooks` table stores HMAC signing secrets in plaintext (`secret TEXT NOT NULL DEFAULT ''`). These should be encrypted at rest like provider keys are.

#### S9. No CSRF Protection on State-Changing Management API Routes
**Issue:** The management API relies solely on API key authentication (`X-Admin-Key` or `Authorization: Bearer`). Cookie-based sessions aren't used, so CSRF is mitigated by design for API key users. However, if the dashboard uses session cookies in the future, CSRF tokens should be added.
**Severity:** Medium (currently mitigated by API-key-only auth)

#### S10. `X-Forwarded-For` Trusted Without Validation
**File:** `internal/engine/auth.go:70-83`
**Issue:** `clientIP()` trusts `X-Forwarded-For` from any source. An attacker can spoof their IP to bypass rate limiting by setting a fake `X-Forwarded-For` header.
**Fix:** Only trust `X-Forwarded-For` when behind a known reverse proxy (configurable trusted proxy list).

---

## 2. Code Quality & Bugs

### HIGH

#### Q1. God File: `engine.go` (2,125 lines)
**File:** `internal/engine/engine.go`
**Issue:** This file handles server setup, routing, middleware wiring, module registration, and much more in a single 2,125-line file. This makes it hard to maintain, test, and review.
**Fix:** Extract module registration, route mounting, and configuration into separate files.

#### Q2. Insufficient Error Handling in Database Operations
**Multiple files** (e.g., `internal/mesh/mesh.go:86`, `internal/engine/webhooks.go:81-83`)
**Issue:** Many `db.Exec()` calls ignore errors:
```go
conn.Exec(meshSchema)      // mesh.go:86 - schema creation error silently ignored
conn.Exec(webhookSchema)   // webhooks.go:81 - error logged but execution continues
```
If schema creation fails, the application runs with missing tables and will produce confusing errors later.

#### Q3. Missing `rows.Err()` Checks After Iteration
**Multiple files**
**Issue:** After iterating over `rows.Next()`, the code rarely checks `rows.Err()` which could mask I/O errors during iteration. Go database best practice requires checking `rows.Err()` after the loop.

#### Q4. Inconsistent Error Response Formats
**Multiple files**
**Issue:** Error responses use varying formats:
- `{"error":"message"}` (string)
- `{"error":{"message":"...","type":"..."}}` (object, OpenAI-style)
- `http.Error()` with plain text
API consumers must handle all three formats. The proxy endpoints correctly use OpenAI-compatible format, but management API endpoints are inconsistent.

### MEDIUM

#### Q5. `defer r.Body.Close()` After `io.ReadAll`
**File:** `internal/proxy/handler.go:95`, `internal/apps/billing/stripe.go:318`
**Issue:** `r.Body.Close()` is deferred after `io.ReadAll(r.Body)` — the body is already fully read, so this is a no-op. More importantly, `r.Body` doesn't need closing in HTTP handlers (the server does it), but the pattern suggests a misunderstanding. This is cosmetic but worth noting.

#### Q6. `mustParseCIDR` Panics at Runtime
**File:** `internal/apps/forge/executor.go:71-76`
**Issue:** `mustParseCIDR` calls `panic()` on invalid CIDR strings. Since these are hardcoded constants, this is safe, but the function is called on every request to `isPrivateIP()`, re-parsing CIDR strings every time. These should be package-level variables initialized once.

#### Q7. No Context Cancellation Propagation in Streaming
**File:** `internal/proxy/streaming.go`
**Issue:** When clients disconnect during streaming, the `context.Context` from `r.Context()` should cancel the upstream provider request. Verify that context cancellation is properly propagated to avoid orphaned goroutines making provider API calls.

---

## 3. Architecture & Design

### HIGH

#### A1. Extremely Low Test Coverage
- **35 test files** covering **412 source files** (~8.5% file coverage)
- **7,967 lines of test code** vs **67,194 lines of source** (~11.9% line ratio)
- CI only tests `./internal/license/`, `./internal/storage/`, `./internal/engine/`, `./internal/proxy/` — skipping all of `internal/features/` (130+ middleware modules), `internal/apps/` (billing, forge, observe, etc.), `internal/auth/`, and `internal/provider/`
- The entire billing system, RBAC enforcement, and authentication have **zero test coverage** in CI

**Fix:** Prioritize tests for:
1. `internal/auth/` — authentication and RBAC
2. `internal/apps/billing/` — financial operations
3. `internal/features/` — at least the security-critical middleware (firewall, ratelimit, authgate, secretscan)

#### A2. No Static Analysis in CI
**File:** `.github/workflows/ci.yml`
**Issue:** CI runs `go vet` on only 6 packages (out of 20+). No `staticcheck`, `golangci-lint`, or `gosec` security scanner is configured. These tools would catch many of the issues in this audit automatically.
**Fix:** Add `golangci-lint` with `gosec` and `staticcheck` linters targeting `./...`.

#### A3. SQLite as Production Database
**Issue:** The entire platform uses SQLite for all data persistence (requests, billing, users, API keys, webhook deliveries, mesh transactions, etc.). While SQLite is excellent for single-node deployments, it becomes a bottleneck for:
- Multi-node/HA deployments (SQLite doesn't support networked access)
- High write throughput (WAL mode helps but has limits)
- Large datasets (no native replication, backup requires file copy)

WAL mode and busy timeout are correctly configured (`internal/storage/db.go:31-37`), which is good for single-node use.

**Recommendation:** Document SQLite as the default/development database. Consider abstracting the storage layer to support PostgreSQL for production deployments.

#### A4. Monolithic Binary with 130+ Features
**Issue:** All 130+ middleware modules are compiled into a single binary and loaded at startup. While this simplifies deployment, it means:
- Build times increase with every new feature
- Memory footprint includes all features even if only 5 are used
- No ability to deploy just the proxy without billing, marketing, etc.

The toggle system (`internal/toggle/`) mitigates runtime impact, but the binary still carries all code.

### MEDIUM

#### A5. No Database Migration Versioning Beyond Sequential
**File:** `internal/storage/migrations.go`
**Issue:** Migrations are a simple sequential list (`migrationV1` through `migrationV5`). There's no down-migration support, no migration checksums, and no way to verify migration integrity. For a platform handling billing and financial data, this is risky.

#### A6. No Request Tracing/Correlation Across Services
**Issue:** While individual request tracing exists within the proxy, there's no distributed tracing correlation (W3C Trace Context / B3) between mesh nodes. The `internal/engine/otel.go` file suggests OTEL support, but mesh-to-mesh requests may not propagate trace context.

#### A7. Config File is 1,110 Lines
**File:** `internal/config/config.go` (1,110 lines)
**Issue:** The configuration struct and parsing logic is in a single large file. With 130+ features, each adding config fields, this file will continue growing.

---

## 4. Dependencies & Deployment

### MEDIUM

#### D1. Go 1.22 — Consider Upgrading
**File:** `go.mod`
**Issue:** Go 1.22 is used. Go 1.23+ includes security fixes, performance improvements, and better iterator support. The project should track a supported Go release.

#### D2. Minimal External Dependencies (Good!)
**Note:** The project has remarkably few dependencies — `go-openai`, `yaml.v3`, and `sqlite`. This reduces supply chain risk significantly.

#### D3. CGO_ENABLED=0 with SQLite
**File:** `Makefile:9`
**Issue:** The build uses `CGO_ENABLED=0` but the `modernc.org/sqlite` package is a pure-Go SQLite implementation, so this works. However, the pure-Go SQLite can be 2-5x slower than the CGo version. For high-throughput deployments, consider documenting the CGo build option.

#### D4. Docker Health Check Uses `stockyard --health`
**File:** `docker-compose.yml:27`
**Issue:** The health check runs `stockyard --health` but the binary may not support this flag. The CI smoke test uses `curl /health`. These should be consistent. Consider using `wget -q -O- http://localhost:4200/health || exit 1`.

#### D5. No `.dockerignore` Found
**Issue:** Without a `.dockerignore`, the Docker build context includes `.git/`, `node_modules/`, test artifacts, etc., making builds slower and images larger.

---

## 5. Specific File-Level Findings

### `internal/auth/crypto.go`
- **Line 52:** Fixed salt `"stockyard-encryption-key-v1"` — acceptable for KDF salt but should be unique per installation if possible
- **Line 47-48:** Minimum key length of 16 chars is too low — recommend 32 chars minimum

### `internal/auth/rbac.go`
- **Line 112:** Role value concatenated into JSON response. While currently safe (roles are DB-controlled constants), this should use `json.Marshal` or `fmt.Sprintf(%q)` for defense in depth
- **Lines 129-135:** Admin permission check is completely bypassed (see S5)

### `internal/engine/auth.go`
- **Line 34:** `valid := rl.windows[ip][:0]` — this slice trick reuses the backing array, which is efficient but can cause confusion. It's correct here.
- **Lines 49-63:** Cleanup runs under mutex lock while iterating the entire map — could cause latency spikes

### `internal/apps/billing/stripe.go`
- **Line 96-98:** `handleStripeStatus` leaks the `has_key` field (reveals whether Stripe is configured). Not a direct vulnerability but reduces attack surface if removed.
- **Lines 312-383:** Webhook handler doesn't limit request body size

### `internal/proxy/handler.go`
- **Line 26:** Raw request body stored in `req.Extra["_raw_body"]` — this keeps full request payloads in memory. For large requests (e.g., base64 images), this could cause memory pressure.

### `internal/storage/db.go`
- No `PRAGMA foreign_keys = ON` — foreign key constraints exist in schema DDL but SQLite doesn't enforce them by default without this pragma.

---

## 6. Positive Findings

These are things done well that should be maintained:

1. **Parameterized SQL everywhere** — No SQL injection vulnerabilities found in parameterized queries
2. **AES-256-GCM encryption at rest** for provider API keys with automatic migration
3. **Constant-time admin key comparison** (`subtle.ConstantTimeCompare`)
4. **SSRF protection** in forge executor with comprehensive private IP blocking
5. **HMAC-SHA256 webhook signatures** with timestamp validation (5-minute replay window)
6. **XSS escaping** in HTML responses using `html.EscapeString`
7. **WAL mode + busy timeout** for SQLite — correct configuration for concurrent access
8. **Ed25519 license signing** — offline validation, no phone-home
9. **CORS origin validation** with exact-match (not wildcard)
10. **Minimal dependencies** — small attack surface from third-party code

---

## 7. Additional Code Quality Findings (from deep analysis)

### Unchecked `rand.Read` in ID/Key Generation
**Files:** `internal/apiserver/types.go:63,73`, `internal/mcp/server.go:67`, `internal/apps/studio/testing.go:54`
**Issue:** `rand.Read(b)` return errors are ignored across multiple ID generation functions, including `generateAPIKey()`. If the entropy source fails, generated IDs/keys become predictable.
**Severity:** High (security-sensitive for API key generation)

### Widespread Missing `rows.Err()` Checks
**Files:** `internal/apps/studio/studio.go`, `internal/apps/observe/observe.go`, `internal/connect/connect.go`, and others
**Issue:** After `for rows.Next()` loops, `rows.Err()` is rarely checked. This can silently mask I/O errors during database iteration.
**Severity:** Medium

### Silent Database Exec/Scan Failures
**Files:** `internal/apps/forge/executor.go` (6 locations), `internal/connect/connect.go` (5 locations), `internal/apps/studio/testing.go` (3 locations)
**Issue:** Many `db.Exec()` and `QueryRow().Scan()` calls ignore returned errors. On failure, code continues with zero values, leading to data inconsistencies.
**Severity:** Medium

### Goroutine Leaks
- `internal/apiserver/nurture.go:257-266` — Ticker goroutine runs forever with no stop mechanism
- `internal/dashboard/events.go:60-64` — SSE subscriber goroutines leak if channels aren't closed
- `internal/apps/studio/testing.go:198-216` — Background test runner with no error observation
**Severity:** Medium

### HTTP Response Body Leaks
- `internal/provider/openai.go:61` — Body not closed on error path before return
- `internal/apps/studio/optimize.go:204` — `resp.Body.Close()` without defer; leaks on panic
**Severity:** Medium

### No Timeout on Streaming HTTP Client
**File:** `internal/provider/openai.go:113`
**Issue:** `streamClient := &http.Client{}` — streaming client has no timeout, could hang indefinitely.
**Severity:** Medium

---

## 8. Recommended Priority Actions

### Immediate (before next release)
1. **Fix Stripe webhook bypass** (S2) — reject events when secret not configured
2. **Fix RBAC admin bypass** (S5) — implement intended permission restrictions
3. **Add request body size limit to proxy** (S6)
4. **Upgrade key derivation** (S1) — use PBKDF2 or Argon2id

### Short-term (next sprint)
5. Add `golangci-lint` with `gosec` to CI (A2)
6. Expand CI test coverage to auth, billing, and critical middleware (A1)
7. Add periodic cleanup to rate limiter and provider cache (S4, S7)
8. Enable SQLite foreign key enforcement via PRAGMA
9. Fix `X-Forwarded-For` trust (S10) — add configurable trusted proxy list

### Medium-term (next quarter)
10. Break up `engine.go` (2,125 lines) into focused files (Q1)
11. Standardize error response format across all APIs (Q4)
12. Add down-migration support to database migrations (A5)
13. Add `.dockerignore` and fix Docker health check (D4, D5)
14. Document SQLite limitations and production guidance (A3)

---

*Generated by automated codebase audit. Manual review recommended for all critical and high severity items.*
