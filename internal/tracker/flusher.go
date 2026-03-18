package tracker

import (
	"context"
	"log"
	"time"
)

// SpendStore is the interface for persisting spend data.
type SpendStore interface {
	UpsertSpendRollup(project string, cost float64, tokensIn, tokensOut int) error
	UpsertUserSpendRollup(userID string, cost float64, tokensIn, tokensOut int) error
	UpsertUsageMetering(dimension, dimensionKey string, cost float64, tokensIn, tokensOut int) error
}

// Flusher periodically writes in-memory spend data to persistent storage.
type Flusher struct {
	counter  *SpendCounter
	store    SpendStore
	interval time.Duration
	last      map[string]lastFlushed // last flushed values per project
	lastUsers map[string]lastFlushed // last flushed values per user
}

// lastFlushed tracks what was last successfully written.
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
		counter:   counter,
		store:     store,
		interval:  interval,
		last:      make(map[string]lastFlushed),
		lastUsers: make(map[string]lastFlushed),
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

// flush writes delta spend to the store for each project and user.
func (f *Flusher) flush() {
	f.flushProjects()
	f.flushUsers()
}

func (f *Flusher) flushProjects() {
	all := f.counter.GetAll()
	for project, spend := range all {
		prev := f.last[project]
		costDelta := spend.Today - prev.cost
		tokenInDelta := spend.TokensIn - prev.tokensIn
		tokenOutDelta := spend.TokensOut - prev.tokensOut

		if costDelta <= 0 && tokenInDelta <= 0 && tokenOutDelta <= 0 {
			continue
		}

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
			log.Printf("flusher: project upsert failed for %s after %d attempts: %v", project, maxFlushRetries, err)
			continue
		}
		f.last[project] = lastFlushed{cost: spend.Today, tokensIn: spend.TokensIn, tokensOut: spend.TokensOut}

		// Also populate usage_metering for the "project" dimension
		f.store.UpsertUsageMetering("project", project, costDelta, tokenInDelta, tokenOutDelta)
	}
}

func (f *Flusher) flushUsers() {
	all := f.counter.GetAllUsers()
	for userID, spend := range all {
		prev := f.lastUsers[userID]
		costDelta := spend.Today - prev.cost
		tokenInDelta := spend.TokensIn - prev.tokensIn
		tokenOutDelta := spend.TokensOut - prev.tokensOut

		if costDelta <= 0 && tokenInDelta <= 0 && tokenOutDelta <= 0 {
			continue
		}

		var err error
		for attempt := 0; attempt < maxFlushRetries; attempt++ {
			err = f.store.UpsertUserSpendRollup(userID, costDelta, tokenInDelta, tokenOutDelta)
			if err == nil {
				break
			}
			if attempt < maxFlushRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
		}

		if err != nil {
			log.Printf("flusher: user upsert failed for %s after %d attempts: %v", userID, maxFlushRetries, err)
			continue
		}
		f.lastUsers[userID] = lastFlushed{cost: spend.Today, tokensIn: spend.TokensIn, tokensOut: spend.TokensOut}

		// Also populate usage_metering for the "user" dimension
		f.store.UpsertUsageMetering("user", userID, costDelta, tokenInDelta, tokenOutDelta)
	}
}

// FlushNow triggers an immediate flush (useful for testing).
func (f *Flusher) FlushNow() {
	f.flush()
}
