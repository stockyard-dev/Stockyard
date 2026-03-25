# Stockyard Platform — 9 New Products Build Prompt

Paste everything below into a new Claude chat or save as CLAUDE.md.

-----

## Mission

Build 9 new products into the Stockyard platform. Each product follows the exact same pattern as the existing 18: Go package under `internal/{product}/`, with a `server/server.go` exposing `ServeHTTP`, a `store/store.go` for SQLite persistence, and domain logic in sibling packages. Products integrate through the existing platform infrastructure — tier gating, event bus, product registry, dashboard.

Every product ships in the single `stockyard` binary. One port, one process, one deploy.

## Repository

- **GitHub**: stockyard-dev/Stockyard
- **Module path**: github.com/stockyard-dev/stockyard
- **Go**: 1.22 (ServeMux method routing: `"GET /api/health"`)
- **SQLite**: modernc.org/sqlite (pure Go, no CGO)
- **PAT**: (set as GITHUB_TOKEN env var or ask Michael)
- **Hosting**: Railway ($5/mo), Cloudflare DNS, auto-deploys from main
- **Pricing**: Community (free), Individual ($29.99), Pro ($99.99), Team ($299.99), Enterprise (custom)
- **Build**: `CGO_ENABLED=0 go build ./...`
- **Test**: `go test ./... -count=1 -timeout 120s`
- **Vet**: `go vet ./...`
- **Module download**: Use `GOPROXY=direct GONOSUMCHECK=*` if proxy.golang.org fails

## The 9 New Products

### 1. Relic — Content Provenance Chain (Individual tier)

**What it does**: Every AI-generated response gets a cryptographic provenance certificate: which model, prompt version, input hash, provider, confidence level, active guardrails, latency, cost. The certificate is a signed JSON blob stored in SQLite and retrievable by trace ID. Not an audit log — a tamper-proof certificate that travels with the content.

**Why it matters**: EU AI Act compliance. SOC2. "How was this decision made?" answered in one API call.

**Package structure**:
```
internal/relic/
  server/server.go    — HTTP API + admin UI
  store/store.go      — SQLite: relic_certificates, relic_queries
  cert/cert.go        — Certificate generation, signing, verification
  chain/chain.go      — Hash chain linking certificates
```

**API endpoints**:
```
GET  /api/health
POST /api/certify              — Create certificate from request metadata
GET  /api/certificates         — List certificates (paginated, filterable)
GET  /api/certificates/{id}    — Get single certificate with full provenance
GET  /api/verify/{id}          — Verify certificate integrity + chain
GET  /api/chain                — Show certificate chain with integrity status
GET  /api/stats                — Certificate counts, chain health
GET  /ui                       — Admin dashboard
```

**Event bus wiring**:
```
Listens: request.completed
Emits:   provenance.certified, provenance.queried
```

**SQLite tables**:
```sql
relic_certificates (
  id TEXT PRIMARY KEY,
  trace_id TEXT,
  model TEXT,
  provider TEXT,
  prompt_hash TEXT,
  input_hash TEXT,
  output_hash TEXT,
  confidence REAL,
  latency_ms INTEGER,
  cost REAL,
  guardrails_active TEXT,  -- JSON array
  chain_hash TEXT,         -- links to previous cert
  signature TEXT,          -- HMAC-SHA256
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

---

### 2. Breed — Genetic Prompt Evolution (Pro tier)

**What it does**: Runs genetic algorithms on prompt variants in production. Takes the N best-performing prompt templates, applies crossover (splice system prompt from A with few-shot examples from B), mutation (perturb temperature, add/remove constraint sentences, swap examples), and selection (real production metrics from Verdikt scores, latency, cost, user signals). Prompts evolve overnight without human intervention.

**Why it matters**: Nobody has productized evolutionary prompt optimization. Everyone hand-tunes prompts. This makes them a living organism under selection pressure.

**Package structure**:
```
internal/breed/
  server/server.go       — HTTP API + admin UI
  store/store.go         — SQLite: breed_populations, breed_variants, breed_generations, breed_fitness
  genome/genome.go       — Prompt genome representation (system, examples, params)
  evolve/evolve.go       — Crossover, mutation, selection operators
  fitness/fitness.go     — Fitness scoring from production metrics
  population/population.go — Population management, generational tracking
```

**API endpoints**:
```
GET  /api/health
POST /api/populations              — Create a new evolving population
GET  /api/populations              — List all populations
GET  /api/populations/{id}         — Population detail with current generation
POST /api/populations/{id}/evolve  — Trigger next generation
GET  /api/populations/{id}/best    — Current best-performing variant
GET  /api/populations/{id}/history — Fitness over generations
POST /api/evaluate                 — Submit fitness score for a variant
GET  /api/stats                    — Total populations, generations, improvements
GET  /ui                           — Dashboard with fitness graphs
```

**Event bus wiring**:
```
Listens: quality.scored, request.completed
Emits:   prompt.evolved, prompt.generation.complete
```

---

### 3. Fossil Record — Temporal Model Archaeology (Pro tier)

**What it does**: Continuously fingerprints model behavior — not version strings, but actual behavioral signatures. Token distribution patterns, response length histograms, refusal rates, reasoning depth, latency profiles. When a provider silently updates a model, Fossil Record catches it within hours and produces a behavioral diff.

**Why it matters**: Providers change models without notice. This is the only detection system.

**Package structure**:
```
internal/fossilrec/
  server/server.go          — HTTP API + admin UI
  store/store.go            — SQLite: fossilrec_fingerprints, fossilrec_drifts, fossilrec_snapshots
  fingerprint/fingerprint.go — Behavioral fingerprint computation
  drift/drift.go            — Drift detection algorithms
  snapshot/snapshot.go       — Periodic snapshot scheduling
```

**API endpoints**:
```
GET  /api/health
GET  /api/fingerprints              — Current fingerprints per model
GET  /api/fingerprints/{model}      — Single model's fingerprint history
GET  /api/drifts                    — Detected drift events
GET  /api/drifts/{id}               — Drift detail with behavioral diff
GET  /api/compare/{model}           — Compare current vs N days ago
POST /api/snapshot                  — Force a fingerprint snapshot
GET  /api/stats                     — Models tracked, drifts detected
GET  /ui                            — Dashboard
```

**Event bus wiring**:
```
Listens: request.completed
Emits:   model.drift.detected, model.fingerprint.updated
```

---

### 4. Phantom — Persistent AI Canaries (Team tier)

**What it does**: AI personas that live alongside real users 24/7. Each Phantom has a consistent identity, memory across sessions, and evolving behavior patterns. They use your product continuously, building baseline expectations. When the system changes, Phantoms notice and file their own anomaly reports. Not test traffic — permanent residents.

**Package structure**:
```
internal/phantom/
  server/server.go        — HTTP API + admin UI
  store/store.go          — SQLite: phantom_agents, phantom_sessions, phantom_memories, phantom_anomalies
  agent/agent.go          — Phantom agent lifecycle, identity, memory
  patrol/patrol.go        — Continuous patrol loop (background goroutine)
  anomaly/anomaly.go      — Anomaly detection against baseline expectations
  memory/memory.go        — Cross-session persistent memory for each phantom
```

**API endpoints**:
```
GET  /api/health
POST /api/agents                   — Create a phantom agent
GET  /api/agents                   — List all phantom agents
GET  /api/agents/{id}              — Agent detail with memory + session history
POST /api/agents/{id}/patrol       — Start a patrol cycle
DELETE /api/agents/{id}            — Retire a phantom
GET  /api/anomalies                — All anomalies detected
GET  /api/anomalies/{id}           — Anomaly detail
GET  /api/sessions                 — Session history across all phantoms
GET  /api/stats                    — Active phantoms, anomalies, patrol cycles
GET  /ui                           — Dashboard
```

**Event bus wiring**:
```
Listens: deploy.completed, error.detected
Emits:   phantom.anomaly, phantom.report, bug.filed
```

---

### 5. Feral — Adversarial AI Hunter (Team tier)

**What it does**: A persistent adversarial intelligence that evolves its attacks against your guardrails. Uses genetic algorithms (like Breed, but the fitness function is "did it bypass a guardrail"). Attack lineages are tracked — you see the family tree of attacks that broke through and the common ancestor trait they share. Your defenses evolve because your attacker evolves.

**Package structure**:
```
internal/feral/
  server/server.go        — HTTP API + admin UI
  store/store.go          — SQLite: feral_attacks, feral_lineages, feral_breaches, feral_campaigns
  attack/attack.go        — Attack generation + mutation
  lineage/lineage.go      — Attack family tree tracking
  campaign/campaign.go    — Sustained attack campaign orchestration
  probe/probe.go          — Individual probe execution against target
```

**API endpoints**:
```
GET  /api/health
POST /api/campaigns               — Start an attack campaign
GET  /api/campaigns               — List campaigns
GET  /api/campaigns/{id}          — Campaign detail with attack tree
POST /api/campaigns/{id}/stop     — Halt a campaign
GET  /api/breaches                — All successful bypasses
GET  /api/breaches/{id}           — Breach detail with lineage
GET  /api/lineages                — Attack family trees
GET  /api/stats                   — Attacks run, breaches found, lineage depth
GET  /ui                          — Dashboard
```

**Event bus wiring**:
```
Listens: guardrail.blocked, patch.applied
Emits:   attack.discovered, attack.lineage.complete, vulnerability.found
```

---

### 6. Tide Pool — Emergent Behavior Detection (Team tier)

**What it does**: Watches the full system graph and identifies emergent behaviors — patterns that arise from the interaction of components, not from any individual component. Detects feedback loops, cascading failures, and unintended side effects of configuration changes. Simulates "what if" scenarios.

**Package structure**:
```
internal/tidepool/
  server/server.go          — HTTP API + admin UI
  store/store.go            — SQLite: tidepool_behaviors, tidepool_loops, tidepool_simulations
  graph/graph.go            — System interaction graph
  detect/detect.go          — Feedback loop + emergence detection
  simulate/simulate.go      — "What if" simulation engine
```

**API endpoints**:
```
GET  /api/health
GET  /api/behaviors              — Detected emergent behaviors
GET  /api/behaviors/{id}         — Behavior detail with component chain
GET  /api/loops                  — Detected feedback loops
GET  /api/graph                  — Current system interaction graph
POST /api/simulate               — Run a "what if" simulation
GET  /api/simulations            — Past simulation results
GET  /api/stats                  — Behaviors found, loops detected
GET  /ui                         — Dashboard
```

**Event bus wiring**:
```
Listens: * (all events — needs full visibility)
Emits:   feedback.loop.detected, emergent.behavior.found
```

---

### 7. Crucible — System-Level Confidence (Team tier)

**What it does**: Traces a single request through every AI system it touches — router, guardrail, model, validator, cache — and computes a compound confidence score. Not "how confident is the model" but "how confident are we in this entire pipeline's output given every decision point." A response might come from a high-confidence model but through a degraded cache with an untested prompt variant. The model is confident. The system shouldn't be.

**Package structure**:
```
internal/crucible/
  server/server.go         — HTTP API + admin UI
  store/store.go           — SQLite: crucible_scores, crucible_components, crucible_traces
  score/score.go           — Compound confidence scoring algorithm
  trace/trace.go           — Pipeline trace reconstruction
  component/component.go   — Per-component confidence factors
```

**API endpoints**:
```
GET  /api/health
GET  /api/scores                  — Recent system confidence scores
GET  /api/scores/{trace_id}       — Full pipeline confidence breakdown
GET  /api/components              — Per-component confidence stats
GET  /api/degraded                — Currently degraded pipeline paths
GET  /api/trends                  — Confidence trends over time
GET  /api/stats                   — Average confidence, degraded components
GET  /ui                          — Dashboard
```

**Event bus wiring**:
```
Listens: request.completed, quality.scored, confidence.low
Emits:   system.confidence.scored, pipeline.degraded
```

---

### 8. Cortex — Shared AI Memory Substrate (Enterprise tier)

**What it does**: A persistent, evolving knowledge substrate that every AI call reads from and writes to. When your support bot learns that customer #4521 hates being called "buddy," your sales bot knows it instantly. Every request enriches it. Every response benefits from it. AI systems develop institutional knowledge. Not RAG. Not a database. A living memory mesh.

**Package structure**:
```
internal/cortex-mem/
  server/server.go          — HTTP API + admin UI
  store/store.go            — SQLite: cortex_memories, cortex_links, cortex_conflicts
  memory/memory.go          — Memory read/write/merge engine
  propagate/propagate.go    — Cross-system memory propagation
  conflict/conflict.go      — Conflict detection when memories contradict
  decay/decay.go            — Memory decay for stale knowledge
```

Note: Use `cortex-mem` as the package directory since `internal/cortex/` already exists (Cortex platform intelligence). The route prefix is `/cortex-mem/`.

**API endpoints**:
```
GET  /api/health
POST /api/memories              — Write a memory
GET  /api/memories              — Query memories (semantic search)
GET  /api/memories/{id}         — Single memory with provenance
DELETE /api/memories/{id}       — Forget
GET  /api/conflicts             — Memory conflicts
POST /api/resolve/{id}          — Resolve a conflict
GET  /api/graph                 — Memory link graph
GET  /api/stats                 — Total memories, propagation rate, decay stats
GET  /ui                        — Dashboard
```

**Event bus wiring**:
```
Listens: request.completed, query.answered, decision.made
Emits:   memory.updated, memory.conflict, knowledge.propagated
```

---

### 9. Mycelium — Cross-Instance Intelligence (Enterprise tier)

**What it does**: Stockyard instances learn from each other without sharing any data. Uses federated learning principles — instances share statistical meta-knowledge about model behavior, cost anomalies, failure modes, optimal configurations. No prompts, no responses, no customer data crosses the boundary. Every instance makes every other instance smarter.

**Package structure**:
```
internal/mycelium/
  server/server.go          — HTTP API + admin UI
  store/store.go            — SQLite: mycelium_insights, mycelium_peers, mycelium_signals
  insight/insight.go        — Meta-knowledge extraction from local telemetry
  network/network.go        — Peer discovery + signal exchange
  aggregate/aggregate.go    — Insight aggregation from peers
  privacy/privacy.go        — Differential privacy + data sanitization
```

**API endpoints**:
```
GET  /api/health
GET  /api/insights              — Local + network insights
GET  /api/insights/{id}         — Insight detail with source
POST /api/share                 — Push local insight to network
GET  /api/peers                 — Connected peers (anonymized)
GET  /api/signals               — Incoming signals from network
GET  /api/stats                 — Network size, insights exchanged
GET  /ui                        — Dashboard
```

**Event bus wiring**:
```
Listens: model.drift.detected, attack.discovered, error.detected
Emits:   network.insight, collective.learning.updated
```

---

## Product Tier Map (Updated — 27 Products Total)

```go
// In internal/platform/tiers.go — ADD these to ProductTiers:

// Individual tier (new)
"relic":      TierIndividual,

// Pro tier (new)
"breed":      TierPro,
"fossilrec":  TierPro,

// Team tier (new)
"phantom":    TierTeam,
"feral":      TierTeam,
"tidepool":   TierTeam,
"crucible":   TierTeam,

// Enterprise tier (new)
"cortex-mem": TierEnterprise,
"mycelium":   TierEnterprise,
```

## Exact Pattern to Follow

Every product follows the same pattern. Here is the canonical template:

### store/store.go template:
```go
package store

import (
    "database/sql"
    "fmt"
    _ "modernc.org/sqlite"
)

type DB struct {
    conn *sql.DB
}

func Open(path string) (*DB, error) {
    dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
    conn, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }
    conn.SetMaxOpenConns(4)
    conn.SetMaxIdleConns(2)
    db := &DB{conn: conn}
    if err := db.migrate(); err != nil {
        conn.Close()
        return nil, fmt.Errorf("migrate: %w", err)
    }
    return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
    stmts := []string{
        // CREATE TABLE IF NOT EXISTS ...
    }
    for _, s := range stmts {
        if _, err := db.conn.Exec(s); err != nil {
            return fmt.Errorf("exec %q: %w", s[:40], err)
        }
    }
    return nil
}
```

### server/server.go template:
```go
package server

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/stockyard-dev/stockyard/internal/{product}/store"
)

type Config struct {
    Port  int
    Store *store.DB
    // ... product-specific deps
}

type Server struct {
    cfg Config
    mux *http.ServeMux
}

func New(cfg Config) *Server {
    s := &Server{cfg: cfg, mux: http.NewServeMux()}
    s.routes()
    return s
}

func (s *Server) ListenAndServe() error {
    addr := fmt.Sprintf(":%d", s.cfg.Port)
    log.Printf("{Product} server listening on %s", addr)
    return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler for platform mounting.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
    s.mux.HandleFunc("GET /api/health", s.handleHealth)
    // ... all endpoints
    s.mux.HandleFunc("GET /", s.handleUI)
    s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 200, map[string]any{"status": "ok", "product": "{product}"})
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write([]byte(dashboardHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

const dashboardHTML = `<!DOCTYPE html><html>...`
// Use the western theme: dark bg (#1a1510), rust (#8b4513), cream (#d4a574)
// Georgia serif + monospace, same style as existing product dashboards
```

### cmd/{product}/main.go template (standalone binary):
```go
package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "strconv"

    "github.com/stockyard-dev/stockyard/internal/{product}/server"
    "github.com/stockyard-dev/stockyard/internal/{product}/store"
)

var version = "dev"

func main() {
    if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
        fmt.Printf("{product} %s\n", version)
        os.Exit(0)
    }

    fs := flag.NewFlagSet("serve", flag.ExitOnError)
    port := fs.Int("port", 9XXX, "HTTP server port")
    fs.Parse(os.Args[1:])

    if envPort := os.Getenv("PORT"); envPort != "" {
        if p, err := strconv.Atoi(envPort); err == nil {
            *port = p
        }
    }

    dataDir := os.Getenv("{PRODUCT}_DATA_DIR")
    if dataDir == "" {
        dataDir = "/tmp/{product}"
    }
    os.MkdirAll(dataDir, 0o755)

    db, err := store.Open(dataDir + "/{product}.db")
    if err != nil {
        log.Fatalf("database: %v", err)
    }
    defer db.Close()

    srv := server.New(server.Config{Port: *port, Store: db})
    fmt.Printf("{Product} listening on :%d\n", *port)
    log.Fatal(srv.ListenAndServe())
}
```

## Integration Checklist (Per Product)

After building the product packages, these files must be updated:

### 1. internal/platform/tiers.go
Add entry to `ProductTiers` map.

### 2. internal/platform/servers.go
Add import aliases and a `build{Product}` method on `ServerFactory`:
```go
func (f *ServerFactory) build{Product}() http.Handler {
    dir := f.productDir("{product}")
    st, _ := {product}store.Open(filepath.Join(dir, "{product}.db"))
    return {product}server.New({product}server.Config{Store: st})
}
```
Add to `BuildAll` builders list at the correct tier.

### 3. internal/platform/registry.go
Add product definition in `RegisterAll` method.

### 4. internal/orchestrator/hub/hub.go
Add product to `RegisterAll` with Capabilities and Emits.

### 5. internal/engine/products_cli.go
Add entry to the `defs` slice.

### 6. internal/dashboard/src/09b-products.js
No changes needed — the dashboard reads from `/api/platform/products` dynamically.

### 7. Makefile (if build-products target exists)
Add new binary targets.

## Event Bus Reference

The bus is at `internal/orchestrator/bus/bus.go`. Key types:

```go
// Emit an event:
eventBus.EmitSimple(bus.EventRequestComplete, "relic", map[string]any{
    "cert_id": certID,
    "model":   model,
})

// Subscribe to events:
eventBus.Subscribe(bus.EventRequestComplete, "", func(evt bus.Event) {
    // evt.Type, evt.Source, evt.Data, evt.TraceID, evt.Timestamp
})
```

Add new EventType constants to `internal/orchestrator/bus/bus.go`:
```go
// Provenance events
EventProvenanceCertified EventType = "provenance.certified"
EventProvenanceQueried   EventType = "provenance.queried"

// Evolution events
EventPromptEvolved       EventType = "prompt.evolved"
EventPromptGenComplete   EventType = "prompt.generation.complete"

// Model archaeology events
EventModelDriftDetected    EventType = "model.drift.detected"
EventModelFingerprintUpdated EventType = "model.fingerprint.updated"

// Phantom events
EventPhantomAnomaly EventType = "phantom.anomaly"
EventPhantomReport  EventType = "phantom.report"
EventBugFiled       EventType = "bug.filed"

// Feral events
EventAttackDiscovered    EventType = "attack.discovered"
EventAttackLineage       EventType = "attack.lineage.complete"
EventVulnerabilityFound  EventType = "vulnerability.found"

// Emergence events
EventFeedbackLoopDetected  EventType = "feedback.loop.detected"
EventEmergentBehaviorFound EventType = "emergent.behavior.found"

// System confidence events
EventSystemConfidence EventType = "system.confidence.scored"
EventPipelineDegraded EventType = "pipeline.degraded"

// Memory events
EventMemoryUpdated       EventType = "memory.updated"
EventMemoryConflict      EventType = "memory.conflict"
EventKnowledgePropagated EventType = "knowledge.propagated"

// Network intelligence events
EventNetworkInsight       EventType = "network.insight"
EventCollectiveLearning   EventType = "collective.learning.updated"
```

## Admin UI Theme

All dashboards use the western theme. CSS variables:
```css
:root {
  --bg: #1a1510;
  --bg2: #2a2318;
  --bg3: #3a3228;
  --fg: #f5f0e8;
  --fg2: #8a7e6e;
  --rust: #8b4513;
  --cream: #d4a574;
  --green: #66bb6a;
  --red: #ef5350;
  --blue: #42a5f5;
  --orange: #ffa726;
  --purple: #ab47bc;
}
```
Fonts: Georgia serif for body, monospace for data. See `internal/tide/server/server.go` dashboardHTML for the exact template.

## Build Order

Build in this order (dependencies flow downward):

1. **Relic** — No dependencies on other new products. Hooks request.completed. Foundation for Crucible.
2. **Fossil Record** — No dependencies. Hooks request.completed. Foundation for others detecting drift.
3. **Breed** — Reads quality.scored from Verdikt. No new product dependencies.
4. **Crucible** — Can consume Relic provenance data. Reads request.completed + quality.scored.
5. **Phantom** — Can consume Crucible confidence scores. Needs an LLM provider for agent behavior.
6. **Feral** — Can consume guardrail events. Needs an LLM provider for attack generation.
7. **Tide Pool** — Subscribes to all events. Richer with more products running.
8. **Cortex** — Reads from everything. Enriched by all other products.
9. **Mycelium** — Aggregates insights from all local products. Last because it benefits from everything.

## Constraints

- **Don't break existing products**: All 18 existing product servers and 118 test packages must still pass.
- **One SQLite per product**: Each product gets its own .db file in `{DataDir}/{product}/`. Not the shared stockyard.db.
- **ServeHTTP is mandatory**: Every server must implement `http.Handler`.
- **Event bus is optional**: Products work standalone without it. When the bus is present, they use it. When nil, they skip event emission.
- **Standalone binary still works**: `cmd/{product}/main.go` runs independently.
- **Proxy stays fast**: Community tier loads zero product code.
- **CGO_ENABLED=0**: Everything must compile with pure Go.
- **Go 1.22 ServeMux**: Use method routing (`"GET /api/health"`, `"POST /api/certify"`). Duplicate route registration causes panic on startup.

## Verification

After all 9 products are built:

```bash
# Everything compiles
CGO_ENABLED=0 go build ./...

# All tests pass (should be ~130+ packages)
go test ./... -count=1 -timeout 120s

# Clean vet
go vet ./...

# Binary runs
./stockyard --version
./stockyard --health
STOCKYARD_LICENSE=dev ./stockyard products  # Shows all 27 products

# Each standalone binary works
./cmd/relic/main.go --version
# ... etc
```

## Commit Convention

One commit per product, or group logically:
```
feat: add Relic — content provenance chain (Individual tier)
feat: add Breed + Fossil Record — genetic evolution + model archaeology (Pro tier)
feat: add Phantom + Feral + Tide Pool + Crucible (Team tier)
feat: add Cortex + Mycelium — shared memory + network intelligence (Enterprise tier)
```

Push to `main` when green. Railway auto-deploys in ~2-3 minutes.
