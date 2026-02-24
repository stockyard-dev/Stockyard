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
	last     map[string]float64 // last flushed values per project
}

// NewFlusher creates a new spend flusher.
func NewFlusher(counter *SpendCounter, store SpendStore, interval time.Duration) *Flusher {
	if interval == 0 {
		interval = 5 * time.Second
	}
	return &Flusher{
		counter:  counter,
		store:    store,
		interval: interval,
		last:     make(map[string]float64),
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
		lastVal := f.last[project]
		delta := spend.Today - lastVal
		if delta <= 0 {
			continue
		}

		if err := f.store.UpsertSpendRollup(project, delta, 0, 0); err != nil {
			log.Printf("flusher: upsert failed for %s: %v", project, err)
			continue
		}
		f.last[project] = spend.Today
	}
}

// FlushNow triggers an immediate flush (useful for testing).
func (f *Flusher) FlushNow() {
	f.flush()
}
