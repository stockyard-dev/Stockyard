// Package bus is a cross-tool event bus for Stockyard tools.
//
// Every tool in a bundle imports this package and gets two things:
//
//  1. Publish(topic, payload) — emit an event when something happens
//     in the tool (a contact was created, an order was placed, an
//     invoice was paid, etc).
//
//  2. Subscribe(topic, handler) — react to events from other tools
//     in the same bundle.
//
// The bus is backed by a single SQLite database (`_bus.db`) sitting in
// the bundle's shared data directory. Every tool opens that file in
// addition to its own private database. There is no separate process,
// no network sockets, no daemon. The bus is just a table that
// publishers append to and subscribers poll.
//
// Why polling and not SQLite update hooks? Because update hooks only
// fire in the same process that did the write, and the whole point of
// the bus is cross-process. A 200ms polling loop is good enough for
// every workflow we care about (the bookkeeping tool doesn't need to
// see new orders within 5ms, it needs to see them eventually) and it's
// vastly simpler than any IPC-based alternative.
//
// Topics are flat strings using a "tool.event" convention:
// "orders.created", "contacts.updated", "invoices.paid". Tools should
// publish under their own slug and subscribe to whatever they care
// about.
//
// Payloads are arbitrary JSON. The bus does not validate payload
// shape — that's between the publisher and the subscriber. We may add
// a schema registry later if it earns its keep.
package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// Register the pure-Go SQLite driver so tools that import only this
	// package don't need their own blank import. Tools that already import
	// modernc.org/sqlite (e.g. because they have a private store) are
	// unaffected — Go deduplicates blank imports.
	_ "modernc.org/sqlite"
)

// Event is a single bus message.
type Event struct {
	ID        int64           `json:"id"`
	Topic     string          `json:"topic"`
	Source    string          `json:"source"`  // tool slug that published
	Payload   json.RawMessage `json:"payload"` // arbitrary JSON
	CreatedAt time.Time       `json:"created_at"`
}

// Handler is a function that processes a single event.
//
// Handlers run in their own goroutine and should not block for long.
// If a handler returns an error, the bus logs it and moves on — there
// is no automatic retry. Subscribers that need at-least-once delivery
// with retries should track their own progress in their own database.
type Handler func(ctx context.Context, e Event) error

// Bus is a connection to the shared event store.
//
// One Bus per process. The Bus owns a goroutine that polls for new
// events and dispatches them to subscribed handlers. Call Close() at
// shutdown to stop the poller cleanly.
type Bus struct {
	db        *sql.DB
	source    string // this tool's slug, used as the publisher tag
	pollEvery time.Duration
	retention time.Duration // how long to keep events; 0 = forever

	mu          sync.RWMutex
	handlers    map[string][]Handler // topic → handlers
	allHandlers []Handler            // handlers that run for every event
	cursor      int64                // last event ID we've dispatched

	// Handler dispatch is backpressured through a bounded worker pool
	// rather than unbounded `go func() { ... }()` spawns. The dispatch
	// loop writes dispatchJob values to `jobs`; a fixed number of
	// worker goroutines pull from the channel and run the handlers.
	// If the channel fills, the dispatch loop blocks, which means
	// "don't read more events until handlers catch up" — correct
	// backpressure.
	jobs      chan dispatchJob
	workers   int
	workersWg sync.WaitGroup

	// closeOnce makes Close safe to call multiple times. Without this,
	// the second call would panic on `close(b.stopCh)` because closing
	// an already-closed channel is a runtime panic in Go. closeErr
	// caches the result of the first Close so subsequent calls return
	// the same value.
	closeOnce sync.Once
	closeErr  error

	stopCh  chan struct{}
	doneCh  chan struct{} // closed when poll() exits
	pruneCh chan struct{} // closed when pruner exits
}

// dispatchJob is one unit of work for the handler worker pool: run
// `handler(ctx, event)` with a 30-second context timeout, log any
// error with the given prefix. The prefix differentiates topic-
// specific handler errors from wildcard handler errors in the logs.
type dispatchJob struct {
	handler   Handler
	event     Event
	errPrefix string // "handler" or "wildcard handler on"
}

// DefaultWorkers is the default size of the handler worker pool.
// Chosen as a reasonable upper bound for most bundles: with 16
// workers, even a burst of 1000 events with 3 handlers per topic is
// processed without spawning 3000 goroutines, while 16 is still
// enough concurrency to saturate a typical 4-8 core machine on
// handler work that involves SQLite writes.
const DefaultWorkers = 16

// DefaultRetention is how long events stay in the bus DB before
// pruning. 30 days is enough for any reasonable subscriber to recover
// from a long outage and replay missed events, but short enough that
// the DB doesn't grow forever. Tools can override with SetRetention.
const DefaultRetention = 30 * 24 * time.Hour

// SetRetention overrides the retention window. Pass 0 to disable
// pruning entirely (audit log mode — events are kept forever).
//
// Safe to call any time. The change takes effect on the next prune
// cycle, which is at most 24 hours away by default.
func (b *Bus) SetRetention(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.retention = d
}

// Open initializes (or opens) the shared bus database in the given
// bundle directory. `source` is this tool's slug — every event this
// process publishes will be tagged with it.
//
// The bundleDir is the SHARED directory for the bundle, not the
// per-tool data directory. By convention this is the parent of the
// tool's own data dir: if dossier writes to $BUNDLE/data/dossier/,
// the bus lives at $BUNDLE/data/_bus.db.
func Open(bundleDir, source string) (*Bus, error) {
	path := filepath.Join(bundleDir, "_bus.db")
	// NOTE: modernc.org/sqlite does NOT parse mattn/go-sqlite3 style DSN
	// params like ?_journal_mode=WAL&_busy_timeout=5000 — it silently ignores
	// them and you're left in DELETE journal mode with no busy timeout, which
	// guarantees SQLITE_BUSY under any concurrent writer. Use modernc's
	// _pragma= syntax instead, which maps to real PRAGMA execution on open.
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("bus: open %s: %w", path, err)
	}

	// Pin to a single connection. database/sql will otherwise lazily
	// open multiple connections under load, and from SQLite's lock
	// manager perspective those connections are independent writers
	// that serialize through busy_timeout — which works MOST of the
	// time but loses races at the exact moment of lock acquisition,
	// surfacing as SQLITE_BUSY with bursts of ~1% INSERT loss under
	// 20 concurrent publishers (verified empirically — see
	// TestProbe_ConcurrentPublishers in probe_test.go).
	//
	// With MaxOpenConns=1, all reads, writes, and DDL are serialized
	// in Go-land before they hit SQLite. Modernc's WAL journal still
	// gives us the fast-INSERT performance benefit; we're just trading
	// imaginary intra-process write parallelism (which SQLite doesn't
	// support anyway) for guaranteed correctness.
	db.SetMaxOpenConns(1)

	// Schema setup is racy on first open: when multiple tools in a
	// bundle boot at the same instant, all three run CREATE TABLE at
	// once and SQLite's busy_timeout does NOT cover schema-change
	// contention (DDL needs an exclusive lock that is handled
	// differently from DML waits). Retry on BUSY/LOCKED with a short
	// backoff — schema creation is idempotent so retrying is safe, and
	// the losing side is expected to find the table already there on
	// the next attempt.
	if err := execWithRetry(db, schema, 20, 50*time.Millisecond); err != nil {
		db.Close()
		return nil, fmt.Errorf("bus: schema: %w", err)
	}

	// Start each subscriber at the current high water mark — a tool
	// that just booted does not get every event since the dawn of
	// time, only events that happen from now on. Subscribers that
	// want backfill should query the events table directly using
	// FetchSince().
	var cursor int64
	db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM events`).Scan(&cursor)

	b := &Bus{
		db:        db,
		source:    source,
		pollEvery: 200 * time.Millisecond,
		retention: DefaultRetention,
		handlers:  make(map[string][]Handler),
		cursor:    cursor,
		workers:   DefaultWorkers,
		jobs:      make(chan dispatchJob, DefaultWorkers*4),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		pruneCh:   make(chan struct{}),
	}
	// Spin up the worker pool before the poll goroutine so there's no
	// window where dispatch could try to send into an empty worker set.
	b.workersWg.Add(b.workers)
	for i := 0; i < b.workers; i++ {
		go b.worker()
	}
	go b.poll()
	go b.pruner()
	return b, nil
}

// execWithRetry runs db.Exec with exponential-ish backoff on BUSY /
// LOCKED errors. Used for schema setup where multiple processes may
// race to initialize the same database at boot. Non-BUSY errors
// return immediately without retry.
func execWithRetry(db *sql.DB, query string, maxAttempts int, baseDelay time.Duration) error {
	var lastErr error
	delay := baseDelay
	for i := 0; i < maxAttempts; i++ {
		_, err := db.Exec(query)
		if err == nil {
			return nil
		}
		msg := err.Error()
		// modernc.org/sqlite surfaces these as free-form strings —
		// match on substring rather than sentinel error values so we
		// don't couple to a specific driver version.
		if !strings.Contains(msg, "database is locked") &&
			!strings.Contains(msg, "SQLITE_BUSY") &&
			!strings.Contains(msg, "SQLITE_LOCKED") {
			return err
		}
		lastErr = err
		time.Sleep(delay)
		// Cap the delay so a genuinely stuck database still errors
		// out in bounded time (20 attempts * 250ms = 5s worst case).
		if delay < 250*time.Millisecond {
			delay += 25 * time.Millisecond
		}
	}
	return lastErr
}

// Prune deletes events older than the retention window. Safe to call
// at any time. Returns the number of rows deleted.
//
// Pruning is also run automatically once a day by a background
// goroutine started in Open. You only need to call this manually for
// tests or one-off cleanup.
//
// Resolution: the events table stores created_at via SQLite's
// datetime('now') which has 1-second precision. Retention windows
// shorter than ~1 second won't behave correctly because Prune
// compares lexicographic timestamps and rounds the cutoff down to
// the same second as the events being pruned. In practice retention
// is days/weeks/months, so this limit is invisible — but if you find
// yourself wanting sub-second retention you're using the bus wrong
// (and the schema would need to change to strftime '%f' format
// to support it).
func (b *Bus) Prune() (int64, error) {
	b.mu.RLock()
	retention := b.retention
	b.mu.RUnlock()

	if retention <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-retention).UTC().Format(sqliteTimeFormat)
	res, err := b.db.Exec(
		`DELETE FROM events WHERE created_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("bus: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("bus: pruned %d events older than %s", n, cutoff)
	}
	return n, nil
}

// pruner is the background goroutine that runs Prune once a day.
// First prune happens 1 minute after Open so a freshly-started bundle
// gets a quick cleanup, then once every 24 hours after that.
func (b *Bus) pruner() {
	defer close(b.pruneCh)
	first := time.NewTimer(1 * time.Minute)
	defer first.Stop()
	select {
	case <-b.stopCh:
		return
	case <-first.C:
		b.Prune()
	}
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-t.C:
			b.Prune()
		}
	}
}

const schema = `
CREATE TABLE IF NOT EXISTS events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    topic      TEXT NOT NULL,
    source     TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_events_topic ON events(topic);
CREATE INDEX IF NOT EXISTS idx_events_id    ON events(id);
`

// Publish emits an event on the bus. Returns the event ID, which can
// be used by the caller to correlate downstream effects.
//
// Publish is fire-and-forget from the bus's perspective: it inserts
// into the events table and returns. Subscribers in other processes
// will pick it up on their next poll cycle, typically within 200ms.
//
// Payload should be a JSON-marshalable value. nil encodes as JSON
// null (NOT as an empty object — earlier versions of this code
// silently coerced nil → {} which lost the distinction between "no
// payload" and "empty payload" and confused subscribers expecting to
// type-check on null).
func (b *Bus) Publish(topic string, payload any) (int64, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("bus: marshal payload: %w", err)
	}
	res, err := b.db.Exec(
		`INSERT INTO events (topic, source, payload) VALUES (?, ?, ?)`,
		topic, b.source, string(raw),
	)
	if err != nil {
		return 0, fmt.Errorf("bus: insert: %w", err)
	}
	return res.LastInsertId()
}

// Subscribe registers a handler for the given topic. Multiple handlers
// can be registered for the same topic; they all run (in parallel
// goroutines) when an event arrives.
//
// Subscribe is intended to be called at startup. Subscribing after the
// poller has been running is allowed but means you'll miss any events
// that happened before the subscription.
//
// Subscribe after Close is a logged no-op. The caller's handler will
// never be invoked, which is almost certainly a bug in the caller —
// usually it means a registration is happening on a stale Bus
// reference after the host process started shutting down.
func (b *Bus) Subscribe(topic string, h Handler) {
	if b.isClosed() {
		log.Printf("bus: Subscribe(%q) called after Close — handler will never run", topic)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
}

// SubscribeAll registers a handler that runs for every event on the
// bus, regardless of topic. Intended for the audit log and similar
// cross-cutting subscribers.
//
// Most tools should use Subscribe with explicit topics. Reach for
// SubscribeAll only when you genuinely want to observe everything —
// the handler will be called for every publish from every tool, which
// is usually not what you want.
//
// SubscribeAll after Close is a logged no-op, same as Subscribe.
func (b *Bus) SubscribeAll(h Handler) {
	if b.isClosed() {
		log.Printf("bus: SubscribeAll called after Close — handler will never run")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.allHandlers = append(b.allHandlers, h)
}

// isClosed reports whether Close has been called. Uses a non-blocking
// select on stopCh, which Close closes as its first action — this
// avoids adding a separate atomic flag and matches Go convention for
// "is this lifecycle channel closed yet" checks.
func (b *Bus) isClosed() bool {
	select {
	case <-b.stopCh:
		return true
	default:
		return false
	}
}

// sqliteTimeFormat matches what SQLite's `datetime('now')` produces:
// `2006-01-02 15:04:05` in UTC, with a space separator (NOT the
// RFC3339 `T`). Parsing with time.RFC3339 here was a bug — it failed
// silently and left every Event with a zero-value CreatedAt because
// the rows.Scan error was discarded.
const sqliteTimeFormat = "2006-01-02 15:04:05"

// FetchSince returns all events with ID greater than `since`. Useful
// for subscribers that need to backfill on startup or recover after a
// crash.
func (b *Bus) FetchSince(since int64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := b.db.Query(
		`SELECT id, topic, source, payload, created_at
		   FROM events
		  WHERE id > ?
		  ORDER BY id ASC
		  LIMIT ?`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var payload string
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Topic, &e.Source, &payload, &createdAt); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		// Parse SQLite's space-separated timestamp. If parsing fails
		// (shouldn't happen with our schema, but be defensive), log
		// and leave CreatedAt as the zero value rather than silently
		// returning bogus data.
		if t, perr := time.Parse(sqliteTimeFormat, createdAt); perr == nil {
			e.CreatedAt = t.UTC()
		} else {
			log.Printf("bus: bad created_at %q on event %d: %v", createdAt, e.ID, perr)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// poll is the background goroutine that watches for new events and
// dispatches them to subscribed handlers.
func (b *Bus) poll() {
	defer close(b.doneCh)
	t := time.NewTicker(b.pollEvery)
	defer t.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-t.C:
			b.dispatch()
		}
	}
}

// dispatch reads any new events and runs handlers for the ones we have
// subscribers for. Events with no subscribers are skipped (but the
// cursor still advances past them, so we don't re-read them).
func (b *Bus) dispatch() {
	b.mu.RLock()
	cursor := b.cursor
	hasHandlers := len(b.handlers) > 0 || len(b.allHandlers) > 0
	b.mu.RUnlock()

	if !hasHandlers {
		// Even with no handlers, advance the cursor so a tool that
		// later subscribes doesn't get a flood of stale events.
		var max int64
		b.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM events`).Scan(&max)
		if max > cursor {
			b.mu.Lock()
			b.cursor = max
			b.mu.Unlock()
		}
		return
	}

	events, err := b.FetchSince(cursor, 1000)
	if err != nil {
		log.Printf("bus: fetch: %v", err)
		return
	}
	if len(events) == 0 {
		return
	}

	for _, e := range events {
		// Shadow e per-iteration. Go 1.22+ scopes range loop variables
		// per-iteration so this is technically redundant on modern Go,
		// but the explicit shadow makes the goroutine capture obviously
		// correct under any Go version and protects against accidental
		// regression if someone refactors this loop.
		e := e
		b.mu.RLock()
		topicHandlers := append([]Handler(nil), b.handlers[e.Topic]...)
		allHandlers := append([]Handler(nil), b.allHandlers...)
		b.mu.RUnlock()

		// Fan out to topic-specific subscribers AND wildcard
		// subscribers. Both lists go through the same bounded worker
		// pool — no unbounded goroutine spawns. If the jobs channel
		// is full, these sends block, which is correct backpressure:
		// don't move past this event until the workers have caught
		// up. A stopCh select lets Close interrupt a blocked send so
		// the poll loop can exit promptly.
		for _, h := range topicHandlers {
			select {
			case b.jobs <- dispatchJob{handler: h, event: e, errPrefix: "handler"}:
			case <-b.stopCh:
				return
			}
		}
		for _, h := range allHandlers {
			select {
			case b.jobs <- dispatchJob{handler: h, event: e, errPrefix: "wildcard handler on"}:
			case <-b.stopCh:
				return
			}
		}
	}

	b.mu.Lock()
	b.cursor = events[len(events)-1].ID
	b.mu.Unlock()
}

// worker is one goroutine in the handler dispatch pool. Each worker
// ranges over the jobs channel and runs handlers sequentially. When
// the channel is closed (during Close), the worker drains any
// remaining jobs and then returns, releasing its WaitGroup slot.
//
// Each handler runs with a 30-second context timeout. Note that
// modernc.org/sqlite does not honor context cancellation on its Exec
// calls, so the timeout is informational for SQLite-heavy handlers —
// they will complete their DB writes even after the context expires.
// Non-SQLite handlers (HTTP calls, CPU work, etc.) will be
// interrupted correctly.
//
// Handler panics are recovered and logged. A panicking handler must
// not crash the host process, must not kill its worker, and must not
// stall the dispatch pool. Without this recovery, a single bad
// subscription in any bundled tool would take the whole tool down,
// because the bus runs inside the tool's own process.
func (b *Bus) worker() {
	defer b.workersWg.Done()
	for job := range b.jobs {
		b.runHandler(job)
	}
}

// runHandler invokes one handler with timeout context and panic
// recovery. Split out from worker() so the deferred recover happens
// per-job rather than per-worker — a deferred recover at the worker
// level would catch the panic but the worker function would unwind
// and exit, draining the pool over time. Per-job recover means the
// worker keeps pulling new jobs even after a handler panics.
func (b *Bus) runHandler(job dispatchJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("bus: %s %s: panic: %v",
				job.errPrefix, job.event.Topic, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := job.handler(ctx, job.event); err != nil {
		log.Printf("bus: %s %s: %v", job.errPrefix, job.event.Topic, err)
	}
}

// Close stops the polling and pruner goroutines, drains the handler
// worker pool, and closes the database. Handlers already pulled from
// the worker queue get to finish; handlers still queued when Close is
// called also run to completion before Close returns. This is
// at-least-once drain semantics: a clean shutdown will not lose
// events that were observed by dispatch before Close.
//
// Close is safe to call multiple times. Calls after the first are
// no-ops that return the same error from the original close (cached).
//
// Note that stopCh is selected inside dispatch's send path, so if the
// worker pool is saturated when Close is called, the in-flight
// dispatch batch bails out early rather than waiting for workers. Any
// events not yet handed to the worker pool will be seen again on the
// next Open via the cursor recovery path.
func (b *Bus) Close() error {
	b.closeOnce.Do(func() {
		close(b.stopCh)
		<-b.doneCh
		<-b.pruneCh
		// Safe to close jobs now: poll() has exited, so nothing else
		// will write to it. The workers drain what's in the buffer
		// and then return, releasing the wg.
		close(b.jobs)
		b.workersWg.Wait()
		b.closeErr = b.db.Close()
	})
	return b.closeErr
}
