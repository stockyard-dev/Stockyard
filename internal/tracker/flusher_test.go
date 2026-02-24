package tracker

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockStore struct {
	mu      sync.Mutex
	upserts []upsertCall
}

type upsertCall struct {
	project string
	cost    float64
}

func (m *mockStore) UpsertSpendRollup(project string, cost float64, tokensIn, tokensOut int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserts = append(m.upserts, upsertCall{project, cost})
	return nil
}

func TestFlusherFlushes(t *testing.T) {
	counter := NewSpendCounter()
	store := &mockStore{}
	flusher := NewFlusher(counter, store, 50*time.Millisecond)

	// Add some spend
	counter.Add("project-a", 1.50)
	counter.Add("project-b", 3.00)

	// Start flusher with a short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go flusher.Start(ctx)
	<-ctx.Done()

	// Give flusher time to complete final flush
	time.Sleep(100 * time.Millisecond)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.upserts) < 2 {
		t.Errorf("expected at least 2 upserts, got %d", len(store.upserts))
	}

	// Verify both projects were flushed
	projects := make(map[string]bool)
	for _, u := range store.upserts {
		projects[u.project] = true
	}
	if !projects["project-a"] {
		t.Error("project-a not flushed")
	}
	if !projects["project-b"] {
		t.Error("project-b not flushed")
	}
}

func TestFlusherDeduplicate(t *testing.T) {
	counter := NewSpendCounter()
	store := &mockStore{}
	flusher := NewFlusher(counter, store, 0)

	counter.Add("proj", 2.00)
	flusher.FlushNow()
	flusher.FlushNow() // Second flush with no new spend

	store.mu.Lock()
	defer store.mu.Unlock()

	// Should only have 1 upsert (second flush has 0 delta)
	if len(store.upserts) != 1 {
		t.Errorf("expected 1 upsert, got %d", len(store.upserts))
	}
}
