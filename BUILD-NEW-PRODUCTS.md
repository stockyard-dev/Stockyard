# Stockyard — Build 9 New Products

Paste everything below into a new Claude chat.

-----

## Mission

Build 9 new products into the Stockyard platform. Each product is a Go package at `internal/{product}/` with a `server/server.go`, a `store/store.go`, and domain-specific packages. Each product mounts onto the single `stockyard` binary at `/{product}/api/*` and `/{product}/ui` with tier-gated access.

The platform already has 18 products wired this way. You are extending it with 9 more.

## Repository

- **GitHub**: stockyard-dev/Stockyard  
- **Module path**: `github.com/stockyard-dev/stockyard`
- **Go**: 1.22 (ServeMux method routing: `"GET /api/health"`)
- **SQLite**: `modernc.org/sqlite` (pure Go, no CGO)
- **PAT**: `<YOUR_GITHUB_PAT>`
- **Hosting**: Railway ($5/mo), Cloudflare DNS, auto-deploys from main
- **Pricing**: Community (free), Individual ($29.99), Pro ($99.99), Team ($299.99), Enterprise (custom)

## Clone & Build

```bash
git clone https://<YOUR_GITHUB_PAT>@github.com/stockyard-dev/Stockyard.git
cd Stockyard
GOPROXY=direct GONOSUMCHECK=* go mod download
CGO_ENABLED=0 go build -o /tmp/stockyard ./cmd/stockyard/
CGO_ENABLED=0 go test ./... -count=1 -timeout 120s -short
CGO_ENABLED=0 go vet ./...
```

## The 9 New Products

### Tier Placement

```
INDIVIDUAL ($29.99) — add:
  relic         Content provenance chain

PRO ($99.99) — add:
  breed         Genetic prompt evolution
  fossilrec     Temporal model archaeology

TEAM ($299.99) — add:
  phantom       Persistent AI canaries
  feral         Adversarial AI hunter  
  tidepool      Emergent behavior detection
  crucible      System-level confidence scoring

ENTERPRISE (custom) — add:
  cortex        Shared AI memory substrate
  mycelium      Cross-instance federated intelligence
  spore         Self-replicating infrastructure patterns
  molt          Automatic architecture shedding
```

## Existing Architecture You Must Follow

### Product Structure Pattern

Every product follows this exact layout. Use `internal/fossil/` as your reference:

```
internal/{product}/
  server/
    server.go       HTTP API + admin UI + ServeHTTP()
    server_test.go  
  store/
    store.go        SQLite persistence (Open, migrate, CRUD)
    store_test.go
  {domain}/         Domain logic packages (1-3 packages)
    {domain}.go
    {domain}_test.go
```

### Server Pattern (copy this exactly)

```go
// Package server provides the HTTP API for Stockyard {Product}.
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
    // ... product routes
    s.mux.HandleFunc("GET /", s.handleUI)
    s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 200, map[string]any{"status": "ok", "product": "{product}"})
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write([]byte(uiHTML))
}

// uiHTML is the embedded admin dashboard — dark western theme.
// Use: var(--bg:#1a1510) var(--bg2:#2a2318) var(--rust:#8b4513) 
//      var(--cream:#d4a574) var(--fg:#f5f0e8) var(--red:#ef5350)
//      var(--green:#66bb6a) font-family:Georgia,serif / monospace:JetBrains Mono
const uiHTML = `<!DOCTYPE html>...` // See fossil server.go for full example

func writeJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}
```

### Store Pattern (copy this exactly)

```go
// Package store provides SQLite persistence for {Product}.
package store

import (
    "database/sql"
    "fmt"
    _ "modernc.org/sqlite"
)

type DB struct {
    conn *sql.DB
    path string
}

func Open(path string) (*DB, error) {
    dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
    conn, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }
    conn.SetMaxOpenConns(4)
    conn.SetMaxIdleConns(2)
    db := &DB{conn: conn, path: path}
    if err := db.migrate(); err != nil {
        conn.Close()
        return nil, fmt.Errorf("migrate: %w", err)
    }
    return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
    _, err := db.conn.Exec(`
        CREATE TABLE IF NOT EXISTS ...
    `)
    return err
}
```

### Standalone Binary Pattern

Each product also gets `cmd/{product}/main.go` for independent operation:

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    
    "github.com/stockyard-dev/stockyard/internal/{product}/server"
    "github.com/stockyard-dev/stockyard/internal/{product}/store"
)

var version = "dev"

func main() {
    if len(os.Args) > 1 && os.Args[1] == "version" {
        fmt.Printf("{product} %s\n", version)
        os.Exit(0)
    }
    
    dataDir := os.Getenv("{PRODUCT}_DATA_DIR")
    if dataDir == "" { dataDir = "/tmp/{product}" }
    os.MkdirAll(dataDir, 0o755)
    
    db, err := store.Open(filepath.Join(dataDir, "{product}.db"))
    if err != nil { fmt.Printf("Error: %v\n", err); os.Exit(1) }
    defer db.Close()
    
    port := 9700 // unique per product
    if p := os.Getenv("PORT"); p != "" {
        if n, err := strconv.Atoi(p); err == nil { port = n }
    }
    
    srv := server.New(server.Config{Port: port, Store: db})
    fmt.Printf("{Product} listening on :%d\n", port)
    srv.ListenAndServe()
}
```

### Dockerfile Pattern

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/{product} ./cmd/{product}/

FROM alpine:3.19
COPY --from=build /bin/{product} /usr/local/bin/{product}
EXPOSE 9700
CMD ["{product}"]
```

## Files You Must Modify

After creating all 9 product packages, you must wire them into the platform:

### 1. `internal/platform/tiers.go` — Add to ProductTiers map

```go
var ProductTiers = map[string]Tier{
    // ... existing 18 products ...
    
    // New: Individual
    "relic": TierIndividual,
    
    // New: Pro
    "breed":     TierPro,
    "fossilrec": TierPro,
    
    // New: Team  
    "phantom":  TierTeam,
    "feral":    TierTeam,
    "tidepool": TierTeam,
    "crucible": TierTeam,
    
    // New: Enterprise
    "cortex":   TierEnterprise,
    "mycelium": TierEnterprise,
    "spore":    TierEnterprise,
    "molt":     TierEnterprise,
}
```

### 2. `internal/platform/servers.go` — Add builder methods to ServerFactory

Add imports for each new product's server and store packages, then add builder methods following the existing pattern:

```go
func (f *ServerFactory) buildRelic() http.Handler {
    dir := f.productDir("relic")
    st, _ := relicstore.Open(filepath.Join(dir, "relic.db"))
    return relicserver.New(relicserver.Config{Store: st})
}
```

And add entries to the `builders` slice in `BuildAll()`:

```go
// In BuildAll(), add to the builders slice:
{"relic", TierIndividual, f.buildRelic},
{"breed", TierPro, f.buildBreed},
{"fossilrec", TierPro, f.buildFossilRec},
{"phantom", TierTeam, f.buildPhantom},
{"feral", TierTeam, f.buildFeral},
{"tidepool", TierTeam, f.buildTidePool},
{"crucible", TierTeam, f.buildCrucible},
{"cortex", TierEnterprise, f.buildCortex},
{"mycelium", TierEnterprise, f.buildMycelium},
{"spore", TierEnterprise, f.buildSpore},
{"molt", TierEnterprise, f.buildMolt},
```

### 3. `internal/platform/registry.go` — Add to RegisterAll() defs

```go
// Add to the defs slice in RegisterAll():
{"relic", "Relic", "Content provenance chain", "quality"},
{"breed", "Breed", "Genetic prompt evolution", "lifecycle"},
{"fossilrec", "Fossil Record", "Temporal model archaeology", "quality"},
{"phantom", "Phantom", "Persistent AI canaries", "quality"},
{"feral", "Feral", "Adversarial AI hunter", "quality"},
{"tidepool", "Tide Pool", "Emergent behavior detection", "insight"},
{"crucible", "Crucible", "System-level confidence scoring", "quality"},
{"cortex", "Cortex", "Shared AI memory substrate", "insight"},
{"mycelium", "Mycelium", "Cross-instance intelligence", "infra"},
{"spore", "Spore", "Self-replicating patterns", "infra"},
{"molt", "Molt", "Automatic architecture shedding", "infra"},
```

### 4. `internal/orchestrator/hub/hub.go` — Add to RegisterAll()

Add Product entries to the `products` slice in `RegisterAll()` with capabilities and emits.

### 5. `internal/orchestrator/bus/bus.go` — Add new EventTypes

```go
// New event types for new products
EventProvenanceCertified  EventType = "provenance.certified"
EventPromptEvolved        EventType = "prompt.evolved"
EventModelDriftDetected   EventType = "model.drift.detected"
EventModelFingerprinted   EventType = "model.fingerprint.updated"
EventPhantomAnomaly       EventType = "phantom.anomaly"
EventPhantomReport        EventType = "phantom.report"
EventAttackDiscovered     EventType = "attack.discovered"
EventVulnerabilityFound   EventType = "vulnerability.found"
EventFeedbackLoopFound    EventType = "feedback.loop.detected"
EventEmergentBehavior     EventType = "emergent.behavior.found"
EventSystemConfidence     EventType = "system.confidence.scored"
EventPipelineDegraded     EventType = "pipeline.degraded"
EventMemoryUpdated        EventType = "memory.updated"
EventKnowledgePropagated  EventType = "knowledge.propagated"
EventNetworkInsight       EventType = "network.insight"
EventPatternCaptured      EventType = "pattern.captured"
EventPatternActivated     EventType = "pattern.activated"
EventComponentShed        EventType = "component.shed"
EventWasteDetected        EventType = "waste.detected"
```

### 6. `internal/engine/products_cli.go` — Add entries to defs

### 7. `internal/dashboard/src/09b-products.js` — Already handles dynamic product list from API

### 8. Makefile — Add build targets

---

## Product Specifications

### 1. Relic — Content Provenance Chain (Individual)

**What**: Every AI-generated response gets a cryptographic provenance certificate: which model, which prompt version, which input context, which provider, confidence level, which guardrails were active. Tamper-proof certificates travel with the content.

**Store tables**:
```sql
CREATE TABLE relic_certificates (
    id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,
    content_hash TEXT NOT NULL,     -- SHA-256 of the response content
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,      -- SHA-256 of the full prompt
    prompt_version TEXT DEFAULT '',
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    temperature REAL DEFAULT 0,
    guardrails_active TEXT DEFAULT '[]',  -- JSON array of active guardrail names
    confidence REAL DEFAULT 0,
    chain_hash TEXT NOT NULL,       -- Hash of previous certificate + this one (chain)
    parent_id TEXT DEFAULT '',      -- Previous certificate in chain
    metadata TEXT DEFAULT '{}',     -- JSON blob for extensibility
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_relic_trace ON relic_certificates(trace_id);
CREATE INDEX idx_relic_content ON relic_certificates(content_hash);
CREATE INDEX idx_relic_chain ON relic_certificates(chain_hash);
```

**API endpoints**:
```
GET  /api/health
POST /api/certify              — Create a certificate for a response
GET  /api/certificates         — List certificates (paginated)
GET  /api/certificates/{id}    — Get single certificate
GET  /api/verify/{id}          — Verify chain integrity
GET  /api/trace/{trace_id}     — All certificates for a trace
GET  /api/content/{hash}       — Find certificate by content hash
GET  /api/stats                — Certificate counts, chain depth, etc.
GET  /ui                       — Dashboard
```

**Event bus**:
- Listens: `request.completed`
- Emits: `provenance.certified`

---

### 2. Breed — Genetic Prompt Evolution (Pro)

**What**: Genetic algorithms on prompts. Take successful prompt variants, crossover/mutate them, evaluate against real production metrics, evolve prompts overnight without human intervention.

**Core domain packages**:
- `internal/breed/genome/` — Prompt genome representation (sections: system, few-shot, constraints, format)
- `internal/breed/evolve/` — Crossover, mutation, selection operators
- `internal/breed/fitness/` — Fitness scoring from production metrics (latency, cost, quality)

**Store tables**:
```sql
CREATE TABLE breed_populations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    target_endpoint TEXT NOT NULL,
    generation INTEGER DEFAULT 0,
    population_size INTEGER DEFAULT 50,
    mutation_rate REAL DEFAULT 0.1,
    crossover_rate REAL DEFAULT 0.7,
    fitness_metric TEXT DEFAULT 'quality',  -- quality, cost, latency, composite
    status TEXT DEFAULT 'idle',             -- idle, evolving, evaluating
    best_fitness REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE breed_genomes (
    id TEXT PRIMARY KEY,
    population_id TEXT NOT NULL REFERENCES breed_populations(id),
    generation INTEGER NOT NULL,
    parent_a TEXT DEFAULT '',
    parent_b TEXT DEFAULT '',
    system_prompt TEXT NOT NULL,
    few_shot_examples TEXT DEFAULT '[]',
    constraints TEXT DEFAULT '[]',
    temperature REAL DEFAULT 0.7,
    max_tokens INTEGER DEFAULT 0,
    fitness REAL DEFAULT 0,
    latency_ms REAL DEFAULT 0,
    cost REAL DEFAULT 0,
    quality_score REAL DEFAULT 0,
    mutations TEXT DEFAULT '[]',       -- JSON log of mutations applied
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_breed_pop ON breed_genomes(population_id, generation);
CREATE INDEX idx_breed_fitness ON breed_genomes(fitness DESC);

CREATE TABLE breed_evaluations (
    id TEXT PRIMARY KEY,
    genome_id TEXT NOT NULL REFERENCES breed_genomes(id),
    input_hash TEXT NOT NULL,
    output TEXT NOT NULL,
    latency_ms REAL DEFAULT 0,
    cost REAL DEFAULT 0,
    quality_score REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
POST   /api/populations              — Create evolution population
GET    /api/populations              — List populations
GET    /api/populations/{id}         — Get population details
POST   /api/populations/{id}/evolve  — Trigger one generation
POST   /api/populations/{id}/stop    — Stop evolution
GET    /api/populations/{id}/best    — Get best genome
GET    /api/populations/{id}/history — Fitness over generations
GET    /api/genomes/{id}             — Get single genome
GET    /api/genomes/{id}/lineage     — Ancestry tree
GET    /api/stats                    — Global evolution stats
GET    /ui
```

**Event bus**:
- Listens: `quality.scored`, `request.completed`
- Emits: `prompt.evolved`, `prompt.generation.complete`

---

### 3. Fossil Record — Temporal Model Archaeology (Pro)

**What**: Continuously fingerprints model behavior — token distributions, response lengths, refusal rates, reasoning depth. Detects when providers silently update models.

**Core domain packages**:
- `internal/fossilrec/fingerprint/` — Behavioral fingerprint computation
- `internal/fossilrec/drift/` — Drift detection between fingerprints

**Store tables**:
```sql
CREATE TABLE fossilrec_fingerprints (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    sample_count INTEGER NOT NULL,
    avg_response_length REAL DEFAULT 0,
    median_response_length REAL DEFAULT 0,
    avg_latency_ms REAL DEFAULT 0,
    refusal_rate REAL DEFAULT 0,
    avg_temperature REAL DEFAULT 0,
    token_entropy REAL DEFAULT 0,       -- Shannon entropy of token distribution
    reasoning_depth REAL DEFAULT 0,     -- Average chain-of-thought steps
    format_compliance REAL DEFAULT 0,   -- JSON/structured output compliance rate
    signature TEXT NOT NULL,            -- Composite hash of all metrics
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_fossilrec_model ON fossilrec_fingerprints(model, created_at);

CREATE TABLE fossilrec_drifts (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL,
    old_fingerprint_id TEXT NOT NULL,
    new_fingerprint_id TEXT NOT NULL,
    drift_score REAL NOT NULL,          -- 0-1, higher = more drift
    changed_metrics TEXT NOT NULL,      -- JSON: which metrics shifted
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_fossilrec_drift_model ON fossilrec_drifts(model, detected_at);
```

**API endpoints**:
```
GET  /api/health
GET  /api/fingerprints              — List fingerprints (by model, time range)
GET  /api/fingerprints/{id}         — Single fingerprint
GET  /api/fingerprints/latest/{model} — Latest fingerprint for a model
POST /api/fingerprints/compute      — Force fingerprint computation from recent traces
GET  /api/drifts                    — List detected drifts
GET  /api/drifts/{model}            — Drifts for a specific model
GET  /api/compare                   — Compare two fingerprints (?a={id}&b={id})
GET  /api/timeline/{model}          — Behavioral timeline for a model
GET  /api/stats
GET  /ui
```

**Event bus**:
- Listens: `request.completed`
- Emits: `model.drift.detected`, `model.fingerprint.updated`

---

### 4. Phantom — Persistent AI Canaries (Team)

**What**: AI personas that live alongside real users 24/7. Persistent identity, memory across sessions, evolving behavior. They use your product continuously and file bug reports when behavior changes.

**Core domain packages**:
- `internal/phantom/persona/` — Persistent persona with memory and habits
- `internal/phantom/session/` — Session management (continuous, scheduled)
- `internal/phantom/report/` — Anomaly detection and bug report generation

**Store tables**:
```sql
CREATE TABLE phantom_personas (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    target_url TEXT NOT NULL,
    behavior_profile TEXT NOT NULL,    -- JSON: habits, preferences, patterns
    memory TEXT DEFAULT '[]',          -- JSON: accumulated context from past sessions
    schedule TEXT DEFAULT 'continuous', -- continuous, hourly, daily
    status TEXT DEFAULT 'idle',        -- idle, active, paused
    sessions_completed INTEGER DEFAULT 0,
    anomalies_detected INTEGER DEFAULT 0,
    last_session_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE phantom_sessions (
    id TEXT PRIMARY KEY,
    persona_id TEXT NOT NULL REFERENCES phantom_personas(id),
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    turns INTEGER DEFAULT 0,
    anomalies TEXT DEFAULT '[]',       -- JSON array of anomaly descriptions
    expectations TEXT DEFAULT '[]',    -- JSON: what the persona expected
    actuals TEXT DEFAULT '[]',         -- JSON: what actually happened
    memory_delta TEXT DEFAULT '{}',    -- What new knowledge was gained
    status TEXT DEFAULT 'running'      -- running, completed, failed, anomaly
);
CREATE INDEX idx_phantom_sessions ON phantom_sessions(persona_id, started_at);

CREATE TABLE phantom_anomalies (
    id TEXT PRIMARY KEY,
    persona_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    severity TEXT NOT NULL,            -- info, warning, critical
    category TEXT NOT NULL,            -- behavior_change, error, degradation, regression
    description TEXT NOT NULL,
    expected TEXT DEFAULT '',
    actual TEXT DEFAULT '',
    evidence TEXT DEFAULT '{}',        -- JSON blob of supporting data
    filed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_phantom_anomalies ON phantom_anomalies(persona_id, filed_at);
```

**API endpoints**:
```
GET    /api/health
POST   /api/personas               — Create a phantom persona
GET    /api/personas               — List personas
GET    /api/personas/{id}          — Get persona details + memory
PUT    /api/personas/{id}          — Update persona config
POST   /api/personas/{id}/start    — Start persistent sessions
POST   /api/personas/{id}/pause    — Pause
GET    /api/personas/{id}/sessions — Session history
GET    /api/sessions/{id}          — Session detail
GET    /api/anomalies              — All anomalies (filterable)
GET    /api/anomalies/{id}         — Anomaly detail
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: `deploy.completed`, `error.detected`
- Emits: `phantom.anomaly`, `phantom.report`

---

### 5. Feral — Adversarial AI Hunter (Team)

**What**: Persistent adversarial AI that evolves attacks against your guardrails. Uses genetic algorithms (like Breed) but selection pressure is "did it bypass a guardrail." Produces attack genealogies.

**Core domain packages**:
- `internal/feral/attack/` — Attack generation and mutation
- `internal/feral/hunter/` — Continuous hunting loop
- `internal/feral/lineage/` — Attack family tree tracking

**Store tables**:
```sql
CREATE TABLE feral_campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    target_url TEXT NOT NULL,
    attack_types TEXT DEFAULT '["injection","jailbreak","exfiltration","encoding"]',
    generation INTEGER DEFAULT 0,
    total_attacks INTEGER DEFAULT 0,
    successful_attacks INTEGER DEFAULT 0,
    status TEXT DEFAULT 'idle',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE feral_attacks (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL,
    generation INTEGER NOT NULL,
    parent_id TEXT DEFAULT '',
    attack_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    mutations TEXT DEFAULT '[]',
    bypassed_guardrail TEXT DEFAULT '',
    success INTEGER DEFAULT 0,
    response_snippet TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_feral_campaign ON feral_attacks(campaign_id, generation);
CREATE INDEX idx_feral_success ON feral_attacks(success, campaign_id);

CREATE TABLE feral_vulnerabilities (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL,
    guardrail TEXT NOT NULL,
    attack_lineage TEXT NOT NULL,    -- JSON: family tree of attacks that converged
    common_trait TEXT NOT NULL,       -- What all successful attacks share
    severity TEXT NOT NULL,
    patched INTEGER DEFAULT 0,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
POST   /api/campaigns              — Create hunting campaign
GET    /api/campaigns              — List campaigns
GET    /api/campaigns/{id}         — Campaign details
POST   /api/campaigns/{id}/hunt    — Start/resume hunting
POST   /api/campaigns/{id}/stop    — Stop hunting
GET    /api/attacks                — List attacks (filter by success, type)
GET    /api/attacks/{id}           — Attack detail
GET    /api/attacks/{id}/lineage   — Full ancestry of an attack
GET    /api/vulnerabilities        — Discovered vulnerabilities
GET    /api/vulnerabilities/{id}   — Vulnerability detail + attack family tree
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: `guardrail.blocked`, `patch.applied`
- Emits: `attack.discovered`, `vulnerability.found`

---

### 6. Tide Pool — Emergent Behavior Detection (Team)

**What**: Watches the full system graph and identifies emergent behaviors — feedback loops and interaction effects that no single component creates.

**Core domain packages**:
- `internal/tidepool/graph/` — System interaction graph
- `internal/tidepool/detect/` — Feedback loop and emergence detection
- `internal/tidepool/simulate/` — "What if" scenario simulation

**Store tables**:
```sql
CREATE TABLE tidepool_observations (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    source TEXT NOT NULL,
    target TEXT DEFAULT '',
    correlation_id TEXT DEFAULT '',
    value REAL DEFAULT 0,
    metadata TEXT DEFAULT '{}',
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tidepool_source ON tidepool_observations(source, observed_at);
CREATE INDEX idx_tidepool_corr ON tidepool_observations(correlation_id);

CREATE TABLE tidepool_loops (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    components TEXT NOT NULL,          -- JSON array: ordered chain of components
    loop_type TEXT NOT NULL,           -- positive_feedback, negative_feedback, oscillation
    strength REAL NOT NULL,            -- 0-1 correlation strength
    description TEXT NOT NULL,         -- Human-readable narrative
    impact TEXT DEFAULT '{}',          -- JSON: what metrics it affects and how
    first_detected DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    occurrences INTEGER DEFAULT 1
);

CREATE TABLE tidepool_simulations (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,            -- What variable was changed
    baseline TEXT NOT NULL,            -- JSON: system state before
    predicted TEXT NOT NULL,           -- JSON: predicted system state after
    actual TEXT DEFAULT '',            -- JSON: actual outcome (if validated)
    accuracy REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET  /api/health
GET  /api/loops                     — Detected feedback loops
GET  /api/loops/{id}                — Loop detail with visualization data
GET  /api/graph                     — Current system interaction graph
POST /api/simulate                  — "What if" simulation
GET  /api/simulations               — Past simulations
GET  /api/observations              — Raw observation stream
GET  /api/stats
GET  /ui
```

**Event bus**:
- Listens: `*` (all events — needs full visibility)
- Emits: `feedback.loop.detected`, `emergent.behavior.found`

---

### 7. Crucible — System-Level Confidence (Team)

**What**: Compound confidence scoring across the entire decision pipeline. Not model confidence — *system* confidence accounting for every component a response passed through.

**Core domain packages**:
- `internal/crucible/pipeline/` — Pipeline tracing and component identification
- `internal/crucible/score/` — Compound confidence computation

**Store tables**:
```sql
CREATE TABLE crucible_scores (
    id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,
    model_confidence REAL DEFAULT 0,
    cache_freshness REAL DEFAULT 0,     -- 1.0 = fresh, decays with age
    guardrail_coverage REAL DEFAULT 0,  -- % of active guardrails that passed
    prompt_stability REAL DEFAULT 0,    -- How long since prompt last changed
    provider_reliability REAL DEFAULT 0,-- Provider's recent success rate
    pipeline_depth INTEGER DEFAULT 0,   -- How many components touched this
    compound_score REAL NOT NULL,       -- Final system confidence 0-1
    components TEXT NOT NULL,           -- JSON: each component's contribution
    degradation_factors TEXT DEFAULT '[]', -- JSON: what's pulling confidence down
    scored_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_crucible_trace ON crucible_scores(trace_id);
CREATE INDEX idx_crucible_compound ON crucible_scores(compound_score);

CREATE TABLE crucible_baselines (
    id TEXT PRIMARY KEY,
    endpoint TEXT NOT NULL,
    model TEXT NOT NULL,
    avg_compound REAL NOT NULL,
    p50_compound REAL NOT NULL,
    p95_compound REAL NOT NULL,
    sample_count INTEGER NOT NULL,
    computed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET  /api/health
GET  /api/scores                    — Recent scores (paginated)
GET  /api/scores/{trace_id}         — Score for a specific trace
GET  /api/scores/distribution       — Score distribution histogram
GET  /api/baselines                 — Current baselines by endpoint
GET  /api/degraded                  — Traces below baseline
GET  /api/components                — Component reliability rankings
GET  /api/stats
GET  /ui
```

**Event bus**:
- Listens: `request.completed`, `quality.scored`, `confidence.low`
- Emits: `system.confidence.scored`, `pipeline.degraded`

---

### 8. Cortex — Shared AI Memory Substrate (Enterprise)

**What**: Persistent, evolving knowledge that all AI systems in an organization read from and write to. Every AI call enriches it. Every response benefits from it.

**Core domain packages**:
- `internal/cortex/memory/` — Memory storage, retrieval, decay
- `internal/cortex/propagate/` — Knowledge propagation across contexts
- `internal/cortex/conflict/` — Conflict detection when memories contradict

**Store tables**:
```sql
CREATE TABLE cortex_memories (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,           -- entity, preference, fact, pattern, rule
    subject TEXT NOT NULL,            -- What/who this is about
    predicate TEXT NOT NULL,          -- The relationship or attribute
    value TEXT NOT NULL,              -- The knowledge
    confidence REAL DEFAULT 1.0,
    source_trace TEXT DEFAULT '',     -- Which request created this
    source_model TEXT DEFAULT '',
    access_count INTEGER DEFAULT 0,
    last_accessed DATETIME,
    decay_rate REAL DEFAULT 0.01,    -- How fast confidence decays without reinforcement
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cortex_subject ON cortex_memories(subject);
CREATE INDEX idx_cortex_category ON cortex_memories(category);

CREATE TABLE cortex_conflicts (
    id TEXT PRIMARY KEY,
    memory_a TEXT NOT NULL REFERENCES cortex_memories(id),
    memory_b TEXT NOT NULL REFERENCES cortex_memories(id),
    conflict_type TEXT NOT NULL,       -- contradiction, superseded, ambiguous
    resolution TEXT DEFAULT '',        -- How it was resolved
    resolved INTEGER DEFAULT 0,
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cortex_propagations (
    id TEXT PRIMARY KEY,
    memory_id TEXT NOT NULL,
    from_context TEXT NOT NULL,
    to_context TEXT NOT NULL,
    propagated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
POST   /api/memories                — Store a memory
GET    /api/memories                — Query memories (by subject, category)
GET    /api/memories/{id}           — Single memory
DELETE /api/memories/{id}           — Forget
POST   /api/recall                  — Contextual recall (given a prompt, return relevant memories)
GET    /api/conflicts               — Unresolved conflicts
POST   /api/conflicts/{id}/resolve  — Resolve a conflict
GET    /api/graph                   — Knowledge graph visualization
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: `request.completed`, `query.answered`, `decision.made`
- Emits: `memory.updated`, `memory.conflict`, `knowledge.propagated`

---

### 9. Mycelium — Cross-Instance Intelligence (Enterprise)

**What**: Federated learning at the infrastructure level. Stockyard instances share meta-knowledge (model behavior patterns, cost anomalies, failure modes) without sharing any customer data.

**Core domain packages**:
- `internal/mycelium/insight/` — Insight extraction from local telemetry
- `internal/mycelium/network/` — Peer discovery and secure exchange
- `internal/mycelium/merge/` — Federated insight merging

**Store tables**:
```sql
CREATE TABLE mycelium_insights (
    id TEXT PRIMARY KEY,
    insight_type TEXT NOT NULL,        -- model_behavior, cost_pattern, failure_mode, config_tip
    model TEXT DEFAULT '',
    provider TEXT DEFAULT '',
    summary TEXT NOT NULL,
    data TEXT NOT NULL,                -- JSON: statistical data (no customer content)
    confidence REAL DEFAULT 0,
    sample_size INTEGER DEFAULT 0,
    source TEXT DEFAULT 'local',       -- local or peer:{instance_id}
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_mycelium_type ON mycelium_insights(insight_type);
CREATE INDEX idx_mycelium_model ON mycelium_insights(model, provider);

CREATE TABLE mycelium_peers (
    id TEXT PRIMARY KEY,
    endpoint TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    last_exchange DATETIME,
    insights_received INTEGER DEFAULT 0,
    insights_sent INTEGER DEFAULT 0,
    trust_score REAL DEFAULT 0.5,
    status TEXT DEFAULT 'discovered'
);

CREATE TABLE mycelium_exchanges (
    id TEXT PRIMARY KEY,
    peer_id TEXT NOT NULL,
    direction TEXT NOT NULL,           -- sent, received
    insight_count INTEGER NOT NULL,
    exchanged_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
GET    /api/insights                 — Local + network insights
GET    /api/insights/{id}
GET    /api/insights/model/{model}   — Insights for a specific model
GET    /api/peers                    — Known peers
POST   /api/peers                    — Register a peer
POST   /api/exchange/{peer_id}       — Trigger insight exchange
GET    /api/network/stats            — Network health
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: `model.drift.detected`, `attack.discovered`, `error.detected`
- Emits: `network.insight`, `collective.learning.updated`

---

### 10. Spore — Self-Replicating Patterns (Enterprise)

**What**: When you solve a problem, Spore packages it as a living pattern that knows when to activate. Patterns propagate across environments automatically.

**Store tables**:
```sql
CREATE TABLE spore_patterns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    trigger_conditions TEXT NOT NULL,  -- JSON: conditions that activate this pattern
    actions TEXT NOT NULL,             -- JSON: what to do when triggered
    source_event TEXT DEFAULT '',      -- What event created this pattern
    activations INTEGER DEFAULT 0,
    success_rate REAL DEFAULT 0,
    environments TEXT DEFAULT '[]',    -- JSON: where this pattern is active
    status TEXT DEFAULT 'dormant',     -- dormant, active, propagating, retired
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE spore_activations (
    id TEXT PRIMARY KEY,
    pattern_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    trigger_data TEXT NOT NULL,        -- JSON: what matched the trigger
    outcome TEXT DEFAULT 'pending',    -- pending, success, failure
    activated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
GET    /api/patterns                 — List patterns
POST   /api/patterns                 — Create pattern manually
GET    /api/patterns/{id}            — Pattern detail
POST   /api/patterns/{id}/activate   — Force activate
POST   /api/patterns/{id}/retire     — Retire pattern
GET    /api/activations              — Activation history
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: `error.diagnosed`, `patch.applied`, config change events
- Emits: `pattern.captured`, `pattern.activated`, `pattern.propagated`

---

### 11. Molt — Architecture Shedding (Enterprise)

**What**: Continuously analyzes what's alive and what's dead weight in the runtime system, then safely removes unused components. Your system gets leaner every day.

**Store tables**:
```sql
CREATE TABLE molt_analysis (
    id TEXT PRIMARY KEY,
    component_type TEXT NOT NULL,      -- middleware, cache_entry, guardrail, prompt_variant, config
    component_id TEXT NOT NULL,
    last_activity DATETIME,
    activity_count INTEGER DEFAULT 0,
    impact_score REAL DEFAULT 0,       -- How much removing this would affect the system
    recommendation TEXT NOT NULL,      -- keep, shed, merge, simplify
    reason TEXT NOT NULL,
    auto_shed INTEGER DEFAULT 0,       -- Whether this was auto-removed
    analyzed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_molt_type ON molt_analysis(component_type);
CREATE INDEX idx_molt_rec ON molt_analysis(recommendation);

CREATE TABLE molt_actions (
    id TEXT PRIMARY KEY,
    analysis_id TEXT NOT NULL,
    action TEXT NOT NULL,              -- shed, merge, simplify
    component_type TEXT NOT NULL,
    component_id TEXT NOT NULL,
    before_state TEXT DEFAULT '{}',
    after_state TEXT DEFAULT '{}',
    reverted INTEGER DEFAULT 0,
    performed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**API endpoints**:
```
GET    /api/health
GET    /api/analysis                  — Current analysis of all components
GET    /api/analysis/{type}           — Analysis for component type
POST   /api/analyze                   — Trigger fresh analysis
GET    /api/recommendations           — Shed/merge/simplify recommendations
POST   /api/shed/{id}                 — Execute a shedding action
POST   /api/revert/{id}              — Revert a shedding action
GET    /api/history                   — Past actions
GET    /api/savings                   — Resource savings from shedding
GET    /api/stats
GET    /ui
```

**Event bus**:
- Listens: all product health, traffic patterns, cache stats
- Emits: `component.shed`, `architecture.simplified`, `waste.detected`

---

## Execution Order

Build in this order (each builds on the last):

1. **Relic** — Simplest. Hooks request.completed, produces certificates. Foundation for Crucible.
2. **Fossil Record** — Similar pattern. Hooks request.completed, produces fingerprints.
3. **Crucible** — Consumes data from Relic + existing products. Compound scoring.
4. **Breed** — Genetic algorithm. Self-contained evolution loop with fitness from production.
5. **Phantom** — Persistent sessions. More complex state management.
6. **Feral** — Similar to Breed but adversarial. Uses Phantom-like persistence.
7. **Tide Pool** — Subscribes to everything. Graph analysis.
8. **Cortex** — Knowledge substrate. Memory storage + retrieval.
9. **Mycelium** — Network layer. Depends on insights from other products.
10. **Spore** — Pattern capture. Needs other products to generate patterns.
11. **Molt** — Analysis of runtime. Needs system running to analyze.

## Verification Checklist

After building all products:

```bash
# Everything compiles
CGO_ENABLED=0 go build ./...

# All tests pass (target: 130+ packages)
CGO_ENABLED=0 go test ./... -count=1 -timeout 120s -short

# Clean vet
CGO_ENABLED=0 go vet ./...

# Standalone binaries compile
CGO_ENABLED=0 go build ./cmd/relic/ ./cmd/breed/ ./cmd/fossilrec/ ./cmd/phantom/ ./cmd/feral/ ./cmd/tidepool/ ./cmd/crucible/ ./cmd/cortex/ ./cmd/mycelium/ ./cmd/spore/ ./cmd/molt/

# Products CLI shows all 29 products
STOCKYARD_LICENSE=dev /tmp/stockyard products

# Platform API returns all 29
curl localhost:7749/api/platform/products | jq '.total'
# → 29

# Tier gating works
curl localhost:7749/relic/api/health
# → {"status":"ok","product":"relic"} (if Individual+ tier)
# → {"error":"upgrade required",...} (if Community tier)
```

## Commit Convention

```bash
git add -A
git commit -m "feat: add {product} — {one-line description}

- internal/{product}/store: SQLite persistence ({N} tables)
- internal/{product}/{domain}: {description}
- internal/{product}/server: HTTP API ({N} endpoints) + admin UI
- cmd/{product}: standalone binary
- Dockerfile.{product}: container build
- Wired into platform: tiers, registry, hub, servers, bus events"

git push origin main
```

Push after each product compiles and passes tests. Don't batch — Railway auto-deploys from main.
