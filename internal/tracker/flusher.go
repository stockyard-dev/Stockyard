package tracker

import (
	"context"
	"log"
	"time"
)

// SpendStore is the interface for persisting spend data.
type SpendStore interface {
	UpsertSpendRollup(project string, cost float64, tokensIn, tokensOut int) error
}

// Flusher periodically writes in-memory spend data to persistent storage.
type Flusher struct {
	counter  *SpendCounter
	store    SpendStore
	interval time.Duration
	last     map[string]lastFlushed // last flushed values per project
}

// lastFlushed tracks what was last successfully written for a project.
type lastFlushed struct {
	cost      float64
	tokensIn  int
	tokensOut int
}

// maxFlushRetries is the number of retry attempts for a failed DB write.
const maxFlushRetries = 3

// NewFlusher creates a new spend flusher.
func NewFlusher(counter *SpendCounter, store SpendStore, interval time.Duration) *Flusher {
	if interval == 0 {
		interval = 5 * time.Second
	}
	return &Flusher{
		counter:  counter,
		store:    store,
		interval: interval,
		last:     make(map[string]lastFlushed),
	}
}

// Start begins the periodic flush loop. Call with a cancellable context.
func (f *Flusher) Start(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.flush()
		case <-ctx.Done():
			// Final flush on shutdown
			f.flush()
			return
		}
	}
}

// flush writes delta spend to the store for each project.
func (f *Flusher) flush() {
	all := f.counter.GetAll()
	for project, spend := range all {
		prev := f.last[project]
		costDelta := spend.Today - prev.cost
		tokenInDelta := spend.TokensIn - prev.tokensIn
		tokenOutDelta := spend.TokensOut - prev.tokensOut

		if costDelta <= 0 && tokenInDelta <= 0 && tokenOutDelta <= 0 {
			continue
		}

		// Retry transient DB errors with short backoff
		var err error
		for attempt := 0; attempt < maxFlushRetries; attempt++ {
			err = f.store.UpsertSpendRollup(project, costDelta, tokenInDelta, tokenOutDelta)
			if err == nil {
				break
			}
			if attempt < maxFlushRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
		}

		if err != nil {
			// Don't update f.last — the delta will be retried on the next flush cycle
			log.Printf("flusher: upsert failed for %s after %d attempts: %v", project, maxFlushRetries, err)
			continue
		}
		f.last[project] = lastFlushed{
			cost:      spend.Today,
			tokensIn:  spend.TokensIn,
			tokensOut: spend.TokensOut,
		}
	}
}

// FlushNow triggers an immediate flush (useful for testing).
func (f *Flusher) FlushNow() {
	f.flush()
}
