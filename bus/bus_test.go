package bus

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestPublishSubscribeSameProcess: a smoke test that one Bus can talk
// to itself. Not the real use case but the cheapest test to write.
func TestPublishSubscribeSameProcess(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer b.Close()

	var got atomic.Int64
	var wg sync.WaitGroup
	wg.Add(1)
	b.Subscribe("orders.created", func(ctx context.Context, e Event) error {
		var p struct {
			Total int `json:"total"`
		}
		json.Unmarshal(e.Payload, &p)
		got.Add(int64(p.Total))
		wg.Done()
		return nil
	})

	if _, err := b.Publish("orders.created", map[string]any{"total": 42}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never ran")
	}
	if got.Load() != 42 {
		t.Fatalf("want 42, got %d", got.Load())
	}
}

// TestCrossProcess: the test that actually matters. Two Bus instances
// pointed at the same directory simulate two tool processes. Tool A
// publishes, tool B subscribes, and we verify B sees A's event.
func TestCrossProcess(t *testing.T) {
	dir := t.TempDir()

	// Tool A: the publisher (e.g. orders).
	a, err := Open(dir, "orders")
	if err != nil {
		t.Fatalf("open a: %v", err)
	}
	defer a.Close()

	// Tool B: the subscriber (e.g. ledger).
	b, err := Open(dir, "ledger")
	if err != nil {
		t.Fatalf("open b: %v", err)
	}
	defer b.Close()

	got := make(chan Event, 1)
	b.Subscribe("orders.created", func(ctx context.Context, e Event) error {
		got <- e
		return nil
	})

	// Give B's poller a tick to settle so it sees the event we're
	// about to publish, not events from before its cursor.
	time.Sleep(50 * time.Millisecond)

	if _, err := a.Publish("orders.created", map[string]any{
		"order_id": "ord_123",
		"total":    9999,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case e := <-got:
		if e.Source != "orders" {
			t.Errorf("want source=orders, got %q", e.Source)
		}
		if e.Topic != "orders.created" {
			t.Errorf("want topic=orders.created, got %q", e.Topic)
		}
		var p map[string]any
		json.Unmarshal(e.Payload, &p)
		if p["order_id"] != "ord_123" {
			t.Errorf("want order_id=ord_123, got %v", p["order_id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never received event from publisher")
	}
}

// TestNoSubscribersCursorAdvances: a tool with no subscriptions should
// not get hit with a flood of historical events the first time it
// subscribes. The cursor should advance past unsubscribed events.
func TestNoSubscribersCursorAdvances(t *testing.T) {
	dir := t.TempDir()

	// Tool A publishes a bunch.
	a, _ := Open(dir, "orders")
	defer a.Close()
	for i := 0; i < 5; i++ {
		a.Publish("orders.created", map[string]int{"n": i})
	}

	// Tool B opens late, with no subscribers.
	b, _ := Open(dir, "lurker")
	defer b.Close()
	time.Sleep(300 * time.Millisecond) // let one poll cycle run

	// Now B subscribes. It should NOT get the 5 historical events.
	got := make(chan Event, 10)
	b.Subscribe("orders.created", func(ctx context.Context, e Event) error {
		got <- e
		return nil
	})

	// Publish one more — this is the only one B should see.
	a.Publish("orders.created", map[string]int{"n": 999})

	select {
	case e := <-got:
		var p map[string]int
		json.Unmarshal(e.Payload, &p)
		if p["n"] != 999 {
			t.Errorf("got stale event n=%d, expected only n=999", p["n"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("never got the new event")
	}

	// And there should be no more.
	select {
	case e := <-got:
		t.Errorf("unexpected extra event: %+v", e)
	case <-time.After(300 * time.Millisecond):
		// good, nothing else
	}
}

// TestMultipleHandlersSameTopic: registering two handlers on the same
// topic should fan out — both run for every event.
func TestMultipleHandlersSameTopic(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var c1, c2 atomic.Int32
	b.Subscribe("contacts.created", func(ctx context.Context, e Event) error {
		c1.Add(1)
		return nil
	})
	b.Subscribe("contacts.created", func(ctx context.Context, e Event) error {
		c2.Add(1)
		return nil
	})

	for i := 0; i < 3; i++ {
		b.Publish("contacts.created", nil)
	}

	time.Sleep(500 * time.Millisecond)
	if c1.Load() != 3 || c2.Load() != 3 {
		t.Errorf("want both handlers to fire 3 times, got c1=%d c2=%d", c1.Load(), c2.Load())
	}
}

// TestPrune: Prune should delete events older than the retention
// window and leave newer ones alone. We sidestep the 1-minute first-
// prune timer by calling Prune() directly, which is the explicit
// "for tests or one-off cleanup" escape hatch.
func TestPrune(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer b.Close()

	// Insert two events at different ages by manipulating created_at
	// directly in the DB. Publish() always uses datetime('now'), so
	// we have to backdate manually.
	if _, err := b.Publish("orders.created", map[string]int{"id": 1}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := b.Publish("orders.created", map[string]int{"id": 2}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Backdate the first event to 100 days ago.
	if _, err := b.db.Exec(
		`UPDATE events SET created_at = datetime('now', '-100 days') WHERE id = 1`,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := b.Prune()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 row pruned, got %d", n)
	}

	// The recent event should still be there.
	var count int
	b.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count)
	if count != 1 {
		t.Errorf("want 1 event remaining, got %d", count)
	}
}

// TestPruneZeroRetentionIsNoOp: a Bus with retention=0 should never
// delete anything, even ancient events. Useful for tools that want
// the bus to be a permanent audit log.
func TestPruneZeroRetentionIsNoOp(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	b.SetRetention(0)

	b.Publish("orders.created", nil)
	b.db.Exec(`UPDATE events SET created_at = datetime('now', '-1000 days')`)

	n, err := b.Prune()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 0 {
		t.Errorf("retention=0 should prune nothing, got %d", n)
	}
}

// TestSubscribeAll: a wildcard subscriber receives every event
// regardless of topic, AND topic-specific subscribers still fire
// for their own topics. Both lists fan out independently.
func TestSubscribeAll(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var allCount, ordersCount atomic.Int32

	// Wildcard handler — should see every publish.
	b.SubscribeAll(func(ctx context.Context, e Event) error {
		allCount.Add(1)
		return nil
	})

	// Topic-specific handler — should only see orders.created.
	b.Subscribe("orders.created", func(ctx context.Context, e Event) error {
		ordersCount.Add(1)
		return nil
	})

	b.Publish("orders.created", nil)
	b.Publish("contacts.created", nil)
	b.Publish("invoices.paid", nil)

	time.Sleep(500 * time.Millisecond)

	if allCount.Load() != 3 {
		t.Errorf("wildcard: want 3 events, got %d", allCount.Load())
	}
	if ordersCount.Load() != 1 {
		t.Errorf("orders: want 1 event, got %d", ordersCount.Load())
	}
}

// TestSubscribeAllOnly: a Bus with ONLY a wildcard subscriber (no
// topic handlers) should still dispatch. The earlier
// "no handlers, just advance cursor" optimization must not skip
// events when allHandlers is non-empty.
func TestSubscribeAllOnly(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got atomic.Int32
	b.SubscribeAll(func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	b.Publish("anything.at.all", nil)
	b.Publish("else.entirely", nil)

	time.Sleep(500 * time.Millisecond)
	if got.Load() != 2 {
		t.Errorf("want 2 events through wildcard, got %d", got.Load())
	}
}

// TestWorkerPoolBackpressure: a burst of events with a slow handler
// must not spawn an unbounded number of concurrent handlers. The old
// unbounded `go func() { ... }()` dispatch would run all 200 events
// in parallel; the worker pool caps active handlers at b.workers,
// so the observed peak concurrency stays bounded. We verify:
//
//  1. All events are delivered (nothing dropped).
//  2. Peak concurrent handlers <= b.workers (the pool is actually
//     bounded, not cosmetic).
//  3. The total wall time scales with ceil(N / workers) * perHandler,
//     which proves the pool is serializing — not that we spin up 200
//     goroutines that all finish in ~10ms.
func TestWorkerPoolBackpressure(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer b.Close()

	const n = 200
	const perHandler = 10 * time.Millisecond

	var active atomic.Int32
	var peak atomic.Int32
	var done atomic.Int32

	b.SubscribeAll(func(ctx context.Context, e Event) error {
		cur := active.Add(1)
		// Track the peak observed concurrency.
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		time.Sleep(perHandler)
		active.Add(-1)
		done.Add(1)
		return nil
	})

	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := b.Publish("burst.created", i); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	// Wait for all handlers to complete. Give generous headroom —
	// with 16 workers and 200 events at 10ms each, lower bound is
	// ~125ms of pure handler time; add polling overhead.
	deadline := time.Now().Add(10 * time.Second)
	for done.Load() < n && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	elapsed := time.Since(start)

	if got := done.Load(); got != n {
		t.Fatalf("want %d events handled, got %d", n, got)
	}
	if p := peak.Load(); p > int32(b.workers) {
		t.Errorf("peak concurrent handlers = %d, exceeds worker pool size %d",
			p, b.workers)
	}
	// Sanity: with a bounded pool the run must take at least
	// ceil(n/workers) * perHandler. For n=200, workers=16, perHandler=10ms
	// the floor is ceil(200/16)*10ms = 130ms. Anything much faster means
	// the pool isn't actually serializing.
	floor := time.Duration((n+b.workers-1)/b.workers) * perHandler
	if elapsed < floor/2 {
		t.Errorf("elapsed %v < half the theoretical floor %v — "+
			"pool doesn't appear to be bounding concurrency", elapsed, floor)
	}
}

// TestCloseDrainsPendingJobs: jobs that have been handed to the
// worker pool but not yet run must complete before Close returns.
// Without this guarantee, a clean shutdown silently drops events
// that were observed by dispatch, which would break the
// at-least-once property on restart.
func TestCloseDrainsPendingJobs(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	const n = 50
	var done atomic.Int32
	b.SubscribeAll(func(ctx context.Context, e Event) error {
		// Slow enough that most handlers are still queued or in
		// flight when Close is called.
		time.Sleep(20 * time.Millisecond)
		done.Add(1)
		return nil
	})

	for i := 0; i < n; i++ {
		b.Publish("drain.test", i)
	}
	// Let dispatch pick up some events but not enough time for all
	// handlers to finish — ~1 poll tick plus a couple of handler
	// intervals, so the worker pool is busy when we close.
	time.Sleep(250 * time.Millisecond)

	if err := b.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// After Close returns, all jobs that were pulled from the channel
	// must have run. Jobs that were still in the ring buffer when
	// Close was called also run — the workers drain the channel
	// before exiting.
	if got := done.Load(); got != n {
		t.Errorf("want %d handlers finished after Close, got %d", n, got)
	}
}
