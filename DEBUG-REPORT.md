# Stockyard Debug Report — 2026-03-30

Full static analysis of the codebase. Build/test could not run (no network), so all findings are from code review.

---

## CRITICAL (fix immediately)

### 1. SQL Injection — `engine.go:1383`
**File:** `internal/engine/engine.go:1383`
String concatenation of user-supplied `typ` directly into SQL:
```go
query += " AND type = '" + typ + "'"
```
`typ` comes from `r.URL.Query().Get("type")`. Classic SQL injection.

### 2. Unauthenticated Data Export — `exchange.go:590-620`
**File:** `internal/apps/exchange/exchange.go:151-154, 590-620`
Routes `GET /api/exchange/gate/stats` and `GET /api/exchange/gate/export` are explicitly marked "no auth required" and export all captured customer emails as CSV/JSON to any caller.

### 3. Nil Pointer Panic — `handler.go:266`
**File:** `internal/proxy/handler.go:266`
```go
"content": resp.Choices[0].Message.Content,
```
No bounds check on `resp.Choices`. If provider returns empty choices, this panics. (Line 292 checks `len(resp.Choices) > 0` but line 266 does not.)

### 4. Cache Stats Query Returns Wrong Data — `cache.go:14-17`
**File:** `internal/storage/cache.go:14-17`
```go
SELECT COUNT(*), COALESCE(SUM(hits), 0), COALESCE(SUM(cost_saved * hits), 0)
```
Column 2 is `SUM(hits)` but is scanned into `stats.SizeBytes`. Should be `SUM(size_bytes)` or equivalent. Every cache stats API response has wrong `size_bytes`.

### 5. Azure OpenAI Auth Broken — `openai_compat.go:80-94`
**File:** `internal/provider/openai_compat.go:80-94`
`AzureOpenAI` embeds `*OpenAI` but never overrides `Send()`. Comment says "api-key header instead of Bearer" but `Send()` is inherited from OpenAI which sets `Authorization: Bearer`. All Azure requests fail auth.

### 6. Resource Leak on ReadAll Error — `handler.go:104-110, 317-322`
**File:** `internal/proxy/handler.go:104-110` and `317-322`
```go
r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)
body, err := io.ReadAll(r.Body)
if err != nil {
    writeError(w, http.StatusRequestEntityTooLarge, "...")
    return   // <-- body never closed
}
defer r.Body.Close()  // <-- too late, after the early return
```
`defer r.Body.Close()` must be placed before the error check.

### 7. Error Swallowed on License Creation — `server.go:590`
**File:** `internal/apiserver/server.go:590`
```go
s.db.CreateLicense(rec)
```
Error completely ignored. License may fail to persist but handler returns success and sends confirmation email.

---

## HIGH (data corruption / race conditions)

### 8. ~25 Unchecked `rows.Scan()` Calls
Scan errors silently ignored, returning zero-value/corrupt data. Key locations:
- `engine.go:1099, 1110, 1442, 1491, 1549, 1561, 1622`
- `auth.go:229-236, 403-419, 561-568, 668-684, 818-825, 856-870`
- `recall.go:121, 140, 147, 487, 653`
- `observe.go:208, 212, 231-234, 239-240, 289, 322`
- `reputation.go:179, 206-207, 229-231, 241-244`
- `exchange.go:604`
- `requests.go:73-87`
- `spend.go:47-55, 130-139`

### 9. ~15 Missing `rows.Err()` After Iteration Loops
After `for rows.Next() { ... }`, `rows.Err()` is never checked:
- `auth.go` — ListUsers, ListKeys, ListTeams, ListTeamKeys, ListProviderKeys, GetAllProviderKeys
- `requests.go` — ListRequests
- `spend.go` — GetSpendHistory, GetUserSpendHistory

### 10. ~10 Unchecked `QueryRow().Scan()` Calls
- `engine.go:1128-1129, 1422-1424, 1453-1455, 1498`
- `auth.go:218, 1262, 1267, 1272, 1637, 1641, 1645, 1686`
- `migrations.go:27`, `seed.go:15`

### 11. Unchecked `json.NewDecoder().Decode()` — `engine.go:1187`
Malformed JSON request body silently produces zero-value struct; handler inserts empty event rules.

### 12. Race Condition on Provider Map — `streaming.go:200, handler.go:239`
`s.config.Providers` map iterated without synchronization. If providers are dynamically registered, concurrent map read/write panics Go runtime.

### 13. Race Condition in ToggleExchangeStar — `sqlitedb.go:967-983`
TOCTOU: SELECT COUNT then DELETE/INSERT without transaction. Concurrent requests corrupt star counts.

### 14. Unchecked `json.Decode` in Governance Handlers
- `governance.go:216, 302, 337` — `json.NewDecoder(r.Body).Decode(&req)` errors ignored; handlers proceed with zero-value structs.

### 15. Goroutine Leak — Cleanup Ticker — `cleanup.go:47-55`
Infinite `for range ticker.C` with no context or stop channel. Goroutine runs forever, even if DB is closed.

### 16. Goroutine Leak — Background Scanner — `recall.go:66, 88-92`
`go a.backgroundScanner()` with no context or cancellation. Accumulates resources over time.

### 17. Unchecked `Exec()` Calls (~15 locations)
- `auth.go:588` — UpdateTeam description update
- `seed.go:21-24` — 4 DELETE operations
- `seed.go:110, 120, 150, 164, 177` — INSERT operations
- `recall.go:45-49, 148-151, 161-165, 560, 596, 599, 656`

---

## MEDIUM

### 18. Missing Transactions — `seed.go:13-187`
50+ DB operations (DELETE + INSERT) with no transaction. Partial failure leaves inconsistent demo data.

### 19. Unchecked `json.Marshal` — Multiple Locations
- `sqlitedb.go:718, 725, 732, 740`
- `forge.go:257-258, 290-291, 297-298`

### 20. Ignored `fmt.Sscanf` — `server.go:809, 812`
Non-numeric `limit`/`offset` query params silently default to 0.

### 21. Fire-and-Forget DB Write — `auth.go:355-357`
`go func() { s.db.Exec(UPDATE api_keys SET last_used ...) }()` — no error handling, no context.

### 22. Missing HTTP Status Codes — `features/cache_v2_api.go:39,52,58,64,70,78`
Error JSON responses sent without `w.WriteHeader(4xx/5xx)`, so client receives 200 OK with error body.

### 23. Dashboard: URL Parameter Injection — `dashboard/src/00-utils.js:26`
`team_id` appended to URL without `encodeURIComponent()`.

### 24. Dashboard: SSE No Error Recovery — `dashboard/src/05-observe.js:10`
`EventSource` has `es.onerror=()=>{}` — silent failure, no reconnection.

### 25. Dashboard: Async forEach Race — `dashboard/src/03-proxy.js:58`
`lines.forEach(async ...)` fires all API calls in parallel with no await; reports `lines.length` instead of actual success count.

---

## LOW

### 26. Pricing JSON Unmarshal Ignored — `pricing.go:85`
`_ = json.Unmarshal(...)` — if pricing JSON is corrupt, all pricing silently falls back to per-char estimate.

### 27. Unchecked Type Assertions — ~30 locations
`snap["requests"].(int64)` without `, ok` check in `upgrades.go:210-215`, `optimize.go:345-365`, `ingest.go:147-173`.

### 28. Dashboard: Clipboard Writes Unhandled — `03-proxy.js:57, 05-observe.js:42, 10-settings.js:37`

### 29. Dashboard: Empty Catch in SSE — `09b-products.js:7`
`catch(ex){}` swallows all parse errors silently.

### 30. Dashboard: JSON.parse Without try-catch — `01-components.js:17`
`JSON.parse(ss('sy_dismissed_triggers')||'[]')` — corrupted sessionStorage throws.
