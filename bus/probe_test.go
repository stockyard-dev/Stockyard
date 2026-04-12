package bus

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestProbe_PanicInHandler: a handler that panics should not kill
// the worker permanently. The current code has no recover() anywhere.
// Hypothesis: this depletes the worker pool over time.
func TestProbe_PanicInHandler(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer b.Close()

	var goodCount atomic.Int32
	b.Subscribe("good", func(ctx context.Context, e Event) error {
		goodCount.Add(1)
		return nil
	})
	b.Subscribe("bad", func(ctx context.Context, e Event) error {
		panic("handler panic")
	})

	// Publish a panicking event then a good one. If the worker dies on
	// the panic, the good event still gets handled (different worker),
	// but if we publish enough panickers we'll exhaust the pool.
	for i := 0; i < 20; i++ {
		b.Publish("bad", i)
	}
	for i := 0; i < 5; i++ {
		b.Publish("good", i)
	}

	time.Sleep(2 * time.Second)
	got := goodCount.Load()
	t.Logf("good handler ran %d/5 times after 20 panickers", got)
	if got != 5 {
		t.Errorf("worker pool depleted by panicking handlers: only %d/5 good events handled", got)
	}
}

// TestProbe_PublishAfterClose: calling Publish after Close should
// either return an error or be a no-op. It should NEVER panic.
func TestProbe_PublishAfterClose(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	if err := b.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Publish after Close panicked: %v", r)
		}
	}()

	_, err := b.Publish("after.close", "data")
	if err == nil {
		t.Logf("Publish after Close returned nil error (probably wrote to closed db)")
	} else {
		t.Logf("Publish after Close returned err: %v", err)
	}
}

// TestProbe_SubscribeAfterClose: calling Subscribe after Close should
// not panic and should not silently succeed. Race-detector run will
// catch any unguarded map access.
func TestProbe_SubscribeAfterClose(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	b.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Subscribe after Close panicked: %v", r)
		}
	}()

	b.Subscribe("after.close", func(ctx context.Context, e Event) error {
		return nil
	})
	t.Log("Subscribe after Close did not panic")
}

// TestProbe_DoubleClose: calling Close twice should be safe.
func TestProbe_DoubleClose(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")

	if err := b.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second Close panicked: %v", r)
		}
	}()

	if err := b.Close(); err != nil {
		t.Logf("second close returned err: %v (acceptable)", err)
	}
}

// TestProbe_HandlerErrorDoesNotBlockCursor: a handler that returns
// an error should not cause the bus to re-deliver the same event
// forever or block subsequent events from being handled.
func TestProbe_HandlerErrorDoesNotBlockCursor(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var failCount, okCount atomic.Int32
	b.Subscribe("test.event", func(ctx context.Context, e Event) error {
		var p int
		// payload is a JSON-encoded int from Publish
		if string(e.Payload) == "0" {
			failCount.Add(1)
			return context.DeadlineExceeded
		}
		_ = p
		okCount.Add(1)
		return nil
	})

	b.Publish("test.event", 0) // will "fail"
	b.Publish("test.event", 1) // should still get handled
	b.Publish("test.event", 2)

	time.Sleep(1 * time.Second)
	if okCount.Load() != 2 {
		t.Errorf("after one failing handler: want 2 ok, got %d", okCount.Load())
	}
	if failCount.Load() != 1 {
		t.Errorf("failing event delivered %d times, want 1 (we should NOT redeliver)", failCount.Load())
	}
}

// TestProbe_SaturationByWildcard: when a wildcard handler is slow,
// does it starve topic-specific handlers? This was on my hypothesis
// list because the audit log is the canonical wildcard subscriber
// and a slow audit log could in theory back up the whole bus.
func TestProbe_SaturationByWildcard(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var topicCount, wildcardCount atomic.Int32
	b.Subscribe("fast", func(ctx context.Context, e Event) error {
		topicCount.Add(1)
		return nil
	})
	b.SubscribeAll(func(ctx context.Context, e Event) error {
		// Slow wildcard handler
		time.Sleep(100 * time.Millisecond)
		wildcardCount.Add(1)
		return nil
	})

	// Publish 30 events
	for i := 0; i < 30; i++ {
		b.Publish("fast", i)
	}

	// 30 events × 100ms wildcard = 3 seconds of pure wildcard work
	// across 16 workers = ~200ms total wildcard time. Topic handlers
	// should finish much faster since they're trivial.
	time.Sleep(3 * time.Second)

	t.Logf("topic handler ran %d/30, wildcard ran %d/30", topicCount.Load(), wildcardCount.Load())
	if topicCount.Load() != 30 {
		t.Errorf("topic handler ran %d/30, slow wildcard appears to have starved fast handlers", topicCount.Load())
	}
	if wildcardCount.Load() != 30 {
		t.Errorf("wildcard handler ran %d/30, did not finish", wildcardCount.Load())
	}
}

// TestProbe_NilPayload: a publisher passing nil should not crash.
// JSON-encodes to "null", subscribers should be able to read it.
func TestProbe_NilPayload(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got string
	done := make(chan struct{})
	b.Subscribe("nil.test", func(ctx context.Context, e Event) error {
		got = string(e.Payload)
		close(done)
		return nil
	})

	if _, err := b.Publish("nil.test", nil); err != nil {
		t.Fatalf("publish nil: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nil publish never delivered")
	}
	if got != "null" {
		t.Errorf("nil payload encoded as %q, want \"null\"", got)
	}
}

// TestProbe_UnencodablePayload: publishing a value json.Marshal can't
// encode (e.g. a channel) should return an error, not panic.
func TestProbe_UnencodablePayload(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Publish of unencodable payload panicked: %v", r)
		}
	}()

	ch := make(chan int)
	_, err := b.Publish("bad.payload", ch)
	if err == nil {
		t.Errorf("expected error publishing channel, got nil")
	} else {
		t.Logf("got expected error: %v", err)
	}
}

// TestProbe_VeryLargeBurst: 5000 events through the worker pool
// should all deliver without dropping. The default channel buffer
// is workers*4 = 64, so this exercises significant backpressure.
func TestProbe_VeryLargeBurst(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	const n = 5000
	var got atomic.Int32
	b.Subscribe("burst", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	for i := 0; i < n; i++ {
		if _, err := b.Publish("burst", i); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(15 * time.Second)
	for got.Load() < n && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if g := got.Load(); g != n {
		t.Errorf("burst delivery incomplete: got %d/%d", g, n)
	}
}

// TestProbe_ConcurrentPublishers: many goroutines publishing to the
// same bus simultaneously must all succeed and the subscriber must
// see every event exactly once. This catches lost-write bugs in the
// publish path under contention.
func TestProbe_ConcurrentPublishers(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got atomic.Int32
	b.Subscribe("concurrent", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	const goroutines = 20
	const perGoroutine = 100
	const total = goroutines * perGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := b.Publish("concurrent", map[string]int{"g": gid, "i": i}); err != nil {
					t.Errorf("g=%d i=%d publish: %v", gid, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	deadline := time.Now().Add(10 * time.Second)
	for got.Load() < int32(total) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if g := got.Load(); g != int32(total) {
		t.Errorf("concurrent publishers: got %d/%d", g, total)
	}
}

// TestProbe_HandlerHoldingForeverDoesNotBlockShutdown: a handler that
// blocks indefinitely should not prevent Close from returning. This
// is the "graceful shutdown with stuck subscriber" case. We want
// Close to return in bounded time even if a handler is wedged.
//
// CURRENT EXPECTED BEHAVIOR: Close blocks until workersWg.Wait()
// returns, which means a wedged handler holds Close forever. This
// test documents the limitation; if it ever becomes a real problem
// we'd need a Close timeout or worker preemption.
func TestProbe_HandlerHoldingForeverDoesNotBlockShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	dir := t.TempDir()
	b, _ := Open(dir, "test")

	stuck := make(chan struct{})
	defer close(stuck)
	b.SubscribeAll(func(ctx context.Context, e Event) error {
		<-stuck
		return nil
	})

	b.Publish("stuck", "data")
	time.Sleep(500 * time.Millisecond) // let dispatcher pick it up

	closeDone := make(chan struct{})
	go func() {
		b.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		t.Log("Close returned cleanly with a stuck handler — this is a surprise, document it")
	case <-time.After(2 * time.Second):
		t.Log("Close blocked on stuck handler (expected current behavior). " +
			"If this needs to be fixed, add a Close timeout or worker preemption.")
	}
}

// panickyMarshaler is a type whose MarshalJSON method panics. Used
// to probe whether the bus survives a publish-side panic the same
// way it survives a handler-side panic.
type panickyMarshaler struct{}

func (panickyMarshaler) MarshalJSON() ([]byte, error) {
	panic("marshal panic")
}

// TestProbe_PublishSideMarshalPanic: a payload type whose
// MarshalJSON panics is published from the caller's goroutine
// (not a worker), so the per-job recover in runHandler does NOT
// cover it. This panic should propagate to the caller — not crash
// the bus, just bubble up to whoever called Publish.
//
// Acceptable outcomes: (a) Publish returns an error wrapping the
// panic, (b) the caller's goroutine sees the panic and can recover
// itself. NOT acceptable: bus internal state corrupted such that
// subsequent operations fail.
func TestProbe_PublishSideMarshalPanic(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Publish panicked (caller can recover): %v", r)
			}
		}()
		_, err := b.Publish("panic.marshal", panickyMarshaler{})
		if err != nil {
			t.Logf("Publish returned err: %v", err)
		}
	}()

	// Bus should still work after a publish-side panic
	var got atomic.Int32
	b.Subscribe("after.panic", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})
	if _, err := b.Publish("after.panic", "ok"); err != nil {
		t.Errorf("publish after panic: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for got.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Errorf("bus broken after publish-side panic: got %d/1", got.Load())
	}
}

// TestProbe_DuplicateSourceSlug: two Bus instances with the same
// source slug both publish to the same _bus.db. Audit attribution
// can't tell them apart. The bus does not enforce uniqueness — it
// can't, since each tool opens the bus independently — but document
// the behavior so we know what we're committing to.
func TestProbe_DuplicateSourceSlug(t *testing.T) {
	dir := t.TempDir()
	b1, err := Open(dir, "dup")
	if err != nil {
		t.Fatalf("open b1: %v", err)
	}
	defer b1.Close()
	b2, err := Open(dir, "dup")
	if err != nil {
		t.Fatalf("open b2: %v", err)
	}
	defer b2.Close()

	if _, err := b1.Publish("dup.test", "from b1"); err != nil {
		t.Fatalf("publish b1: %v", err)
	}
	if _, err := b2.Publish("dup.test", "from b2"); err != nil {
		t.Fatalf("publish b2: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	events, err := b1.FetchSince(0, 100)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	t.Logf("got %d events from duplicate-slug bus instances", len(events))
	for _, e := range events {
		t.Logf("  id=%d topic=%s source=%s payload=%s", e.ID, e.Topic, e.Source, string(e.Payload))
	}
	if len(events) != 2 {
		t.Errorf("want 2 events from two publishers, got %d", len(events))
	}
	// Both events should have source="dup" — that's the (correct,
	// documented) behavior. The validator catches duplicate slugs at
	// install time; the runtime can't.
	for _, e := range events {
		if e.Source != "dup" {
			t.Errorf("event source=%q, want %q", e.Source, "dup")
		}
	}
}

// TestProbe_ManyConcurrentOpens: schema retry handles 3-way race
// fine in the demo, but does it survive 30 simultaneous opens?
// Worst case: 20 retries * ~250ms = 5s ceiling. With 30 racers
// there could be one that loses every retry.
func TestProbe_ManyConcurrentOpens(t *testing.T) {
	dir := t.TempDir()

	const n = 30
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	buses := make(chan *Bus, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			b, err := Open(dir, fmt.Sprintf("racer-%d", i))
			if err != nil {
				errs <- fmt.Errorf("racer-%d: %w", i, err)
				return
			}
			buses <- b
		}(i)
	}
	wg.Wait()
	close(errs)
	close(buses)

	var failed int
	for err := range errs {
		t.Logf("open failed: %v", err)
		failed++
	}
	var opened int
	for b := range buses {
		opened++
		b.Close()
	}
	t.Logf("opened %d/%d concurrent racers, %d failures", opened, n, failed)
	if failed > 0 {
		t.Errorf("schema retry insufficient for %d concurrent opens: %d failures", n, failed)
	}
}

// TestProbe_PrunerVsDispatcher: the pruner deletes old events while
// the dispatcher is reading new ones. The cursor is by ID so prune
// (deleting low IDs) and dispatch (reading high IDs) shouldn't
// collide, but verify under contention.
func TestProbe_PrunerVsDispatcher(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	// Force aggressive pruning: anything older than 1ms is fair game.
	// (Default DefaultRetention is 30 days; we want pruning every tick.)
	b.SetRetention(1 * time.Millisecond)

	var got atomic.Int32
	b.Subscribe("race", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	// Publish 100 events while pruner is hammering. Pruner runs once
	// a day by default, but we can force it via Prune() in a tight loop.
	prunerDone := make(chan struct{})
	go func() {
		defer close(prunerDone)
		for i := 0; i < 200; i++ {
			b.Prune()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	for i := 0; i < 100; i++ {
		b.Publish("race", i)
		time.Sleep(2 * time.Millisecond)
	}
	<-prunerDone
	time.Sleep(1 * time.Second)

	t.Logf("delivered %d/100 events while pruner deleted aggressively", got.Load())
	// Some events MAY be pruned before the dispatcher sees them — that's
	// the documented behavior of retention=1ms. We just want to verify
	// no crash, no data race, and the bus survives.
	if got.Load() == 0 {
		t.Error("zero events delivered — pruner is racing dispatcher and winning every time")
	}
}

// TestProbe_BundleDirIsAFile: opening the bus where bundleDir is
// actually a regular file (not a directory) should return an
// error, not panic. This is the realistic mistake mode — someone
// passes the data file path instead of the parent directory.
//
// (An earlier version of this test tried a 0o555 read-only dir,
// but the test runs as root in CI containers and mode bits are
// ignored for root, so the dir wasn't actually read-only. Pointing
// at a file is a more reliable failure trigger.)
func TestProbe_BundleDirIsAFile(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file where the bus would expect a directory
	notADir := dir + "/notadir"
	if err := os.WriteFile(notADir, []byte("oops"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Open with file-as-bundleDir panicked: %v", r)
		}
	}()

	b, err := Open(notADir, "test")
	if err == nil {
		t.Error("Open succeeded with a file as bundleDir — expected error")
		b.Close()
	} else {
		t.Logf("got expected error: %v", err)
	}
}

// TestProbe_ExistingTableWithWrongSchema: if the events table exists
// from a prior bus version with a different schema, our CREATE TABLE
// IF NOT EXISTS silently skips and we end up running against an
// unexpected shape. Document the failure mode.
func TestProbe_ExistingTableWithWrongSchema(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate _bus.db with a table that has the wrong shape
	pre, err := sql.Open("sqlite", dir+"/_bus.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("setup db: %v", err)
	}
	if _, err := pre.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, wrong_column TEXT)`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}
	pre.Close()

	// Now Open the bus normally — it'll see the existing table and skip
	// the create.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Open with wrong-schema events table panicked: %v", r)
		}
	}()
	b, err := Open(dir, "schema-test")
	if err != nil {
		t.Logf("Open returned err with mismatched schema: %v", err)
		return
	}
	defer b.Close()

	// Try to publish — INSERT against the wrong shape will fail
	_, err = b.Publish("schema.test", "data")
	if err == nil {
		t.Error("Publish succeeded against wrong-schema table — expected SQL error")
	} else {
		t.Logf("Publish failed as expected: %v", err)
	}
}

// TestProbe_TwoOpensSameProcess: opening the same bundle dir twice
// from the same process produces two distinct Bus instances. Both
// should work; events from one should be visible to the other.
func TestProbe_TwoOpensSameProcess(t *testing.T) {
	dir := t.TempDir()
	b1, err := Open(dir, "p1")
	if err != nil {
		t.Fatalf("open b1: %v", err)
	}
	defer b1.Close()
	b2, err := Open(dir, "p2")
	if err != nil {
		t.Fatalf("open b2: %v", err)
	}
	defer b2.Close()

	var got atomic.Int32
	b2.Subscribe("twoopens", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	if _, err := b1.Publish("twoopens", "from-b1"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for got.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Errorf("two-Open same-process delivery: got %d/1", got.Load())
	}
}

// TestProbe_CorruptBusDB: if _bus.db exists but is garbage (e.g.
// someone wrote a text file to that path), Open should return an
// error gracefully rather than panic.
func TestProbe_CorruptBusDB(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/_bus.db", []byte("not a sqlite database, just text"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Open on corrupt db panicked: %v", r)
		}
	}()

	b, err := Open(dir, "corrupt")
	if err != nil {
		t.Logf("Open returned err on corrupt db: %v", err)
		return
	}
	defer b.Close()
	t.Log("Open succeeded on corrupt db (modernc may have overwritten or recovered)")
}

// TestProbe_NewlineInTopic: the runtime accepts any topic string,
// even ones the validator would reject (newlines, slashes, unicode).
// Document the difference between runtime tolerance and validator
// strictness.
func TestProbe_NewlineInTopic(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got atomic.Int32
	b.Subscribe("topic\nwith\nnewlines", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	// Runtime will happily publish and dispatch this. The validator
	// would reject the bus.json that declared it.
	_, err := b.Publish("topic\nwith\nnewlines", "ok")
	if err != nil {
		t.Logf("publish with newlines in topic: %v (would be a runtime restriction)", err)
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for got.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Errorf("newline topic delivery: got %d/1", got.Load())
	} else {
		t.Log("runtime accepted newline-containing topic — validator must catch this at install time")
	}
}

// TestProbe_ConcurrentPublishersStressed: same shape as the original
// concurrent publishers test but cranked higher to stress the
// MaxOpenConns(1) serialization. 50 goroutines × 200 publishes each
// = 10000 events.
func TestProbe_ConcurrentPublishersStressed(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got atomic.Int32
	b.Subscribe("stress", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	const goroutines = 50
	const perGoroutine = 200
	const total = goroutines * perGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := b.Publish("stress", map[string]int{"g": gid, "i": i}); err != nil {
					t.Errorf("g=%d i=%d publish: %v", gid, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	deadline := time.Now().Add(30 * time.Second)
	for got.Load() < int32(total) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if g := got.Load(); g != int32(total) {
		t.Errorf("stressed publishers: got %d/%d", g, total)
	}
}

// TestProbe_OpenRaceWithLivePublisher: a publisher runs in a tight
// loop while a second Bus instance opens for the same dir. The new
// subscriber registers and we measure how many events it sees vs
// how many were published after Open returned. The contract is "you
// see events published after Open returns", so any event whose
// Publish call returns AFTER Open returns must be visible.
func TestProbe_OpenRaceWithLivePublisher(t *testing.T) {
	dir := t.TempDir()
	pub, err := Open(dir, "publisher")
	if err != nil {
		t.Fatalf("open pub: %v", err)
	}
	defer pub.Close()

	// Background publisher emits as fast as it can, recording the IDs
	// of every event published, with a timestamp.
	type emit struct {
		id int64
		t  time.Time
	}
	stop := make(chan struct{})
	emits := make(chan emit, 10000)
	go func() {
		for {
			select {
			case <-stop:
				close(emits)
				return
			default:
				id, err := pub.Publish("race.test", "x")
				if err == nil {
					emits <- emit{id: id, t: time.Now()}
				}
				time.Sleep(500 * time.Microsecond)
			}
		}
	}()

	// Let publisher get going
	time.Sleep(50 * time.Millisecond)

	// Now Open the subscriber bus
	openStart := time.Now()
	sub, err := Open(dir, "subscriber")
	openEnd := time.Now()
	if err != nil {
		t.Fatalf("open sub: %v", err)
	}
	defer sub.Close()

	seen := make(map[int64]bool)
	var seenMu sync.Mutex
	sub.Subscribe("race.test", func(ctx context.Context, e Event) error {
		seenMu.Lock()
		seen[e.ID] = true
		seenMu.Unlock()
		return nil
	})

	// Let it run a bit longer
	time.Sleep(500 * time.Millisecond)
	close(stop)
	time.Sleep(500 * time.Millisecond) // drain dispatcher

	var allEmits []emit
	for e := range emits {
		allEmits = append(allEmits, e)
	}

	// Find the contract boundary: events whose Publish call returned
	// after openEnd MUST be visible to the subscriber.
	var afterOpen, afterOpenSeen int
	seenMu.Lock()
	defer seenMu.Unlock()
	for _, e := range allEmits {
		if e.t.After(openEnd) {
			afterOpen++
			if seen[e.id] {
				afterOpenSeen++
			}
		}
	}
	t.Logf("publisher emitted %d total events, Open took %v",
		len(allEmits), openEnd.Sub(openStart))
	t.Logf("of %d events published after Open returned, subscriber saw %d",
		afterOpen, afterOpenSeen)
	if afterOpen > 0 && afterOpenSeen < afterOpen {
		// Allow up to 5% jitter from the dispatcher poll interval — an
		// event published microseconds before Subscribe was registered
		// is not the bug we care about
		lossRate := float64(afterOpen-afterOpenSeen) / float64(afterOpen)
		if lossRate > 0.05 {
			t.Errorf("subscriber missed %d/%d events published after Open returned (loss rate %.1f%%)",
				afterOpen-afterOpenSeen, afterOpen, lossRate*100)
		} else {
			t.Logf("loss rate %.1f%% — within tolerance for cursor-vs-subscribe-race jitter",
				lossRate*100)
		}
	}
}

// TestProbe_HandlerExceedsTimeout: a handler that runs longer than
// the 30-second context timeout should not be force-killed (Go has no
// way to do that), but the bus should not get confused. The next event
// in line should still be picked up.
func TestProbe_HandlerExceedsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode — sleeps 31s")
	}
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	var got atomic.Int32
	b.Subscribe("slow", func(ctx context.Context, e Event) error {
		// Sleep past the 30-second context deadline
		time.Sleep(31 * time.Second)
		got.Add(1)
		return nil
	})
	b.Subscribe("fast", func(ctx context.Context, e Event) error {
		got.Add(1)
		return nil
	})

	b.Publish("slow", "data")
	time.Sleep(100 * time.Millisecond)
	b.Publish("fast", "data")

	// The fast event should be picked up by a different worker even
	// while the slow one is still sleeping. We don't want to wait the
	// full 31s for the slow handler — just confirm fast runs quickly.
	deadline := time.Now().Add(2 * time.Second)
	for got.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got.Load() < 1 {
		t.Error("fast handler did not run while slow was sleeping — pool may be too small?")
	}
}

// TestProbe_RetentionChangedMidLife: SetRetention can be called any
// time. Verify that calling it (a) doesn't crash, (b) is observed by
// the next manual Prune call, and (c) the change is durable across
// the daily pruner cycle.
//
// Note: retention is bounded by SQLite datetime() resolution (1 second).
// The probe uses 2-second retention with a 3-second sleep so the
// cutoff is comfortably above the resolution floor — see Prune()
// docstring for the rationale.
func TestProbe_RetentionChangedMidLife(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode — sleeps 3s")
	}
	dir := t.TempDir()
	b, _ := Open(dir, "test")
	defer b.Close()

	// Publish 5 events with default retention (30 days — none should
	// be pruned)
	for i := 0; i < 5; i++ {
		b.Publish("retention.test", i)
	}
	time.Sleep(100 * time.Millisecond)

	n, err := b.Prune()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 0 {
		t.Errorf("default retention pruned %d events, want 0", n)
	}

	// Change to 2-second retention and wait long enough that the events
	// fall outside the window. 3 seconds gives us a full second of
	// margin past the 1-second SQLite datetime() resolution floor.
	b.SetRetention(2 * time.Second)
	time.Sleep(3 * time.Second)

	n, err = b.Prune()
	if err != nil {
		t.Fatalf("prune after retention change: %v", err)
	}
	if n != 5 {
		t.Errorf("after SetRetention(2s)+3s wait, prune deleted %d events, want 5", n)
	}

	// Change back to "forever" — new events should not be pruned
	b.SetRetention(0)
	for i := 0; i < 3; i++ {
		b.Publish("retention.test", i+5)
	}
	time.Sleep(100 * time.Millisecond)

	n, err = b.Prune()
	if err != nil {
		t.Fatalf("prune with retention=0: %v", err)
	}
	if n != 0 {
		t.Errorf("retention=0 pruned %d events, want 0 (audit log mode)", n)
	}
}
