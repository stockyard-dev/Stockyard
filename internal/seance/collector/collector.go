// Package collector queries all connected data sources in parallel and assembles
// a unified view of system state for the synthesizer to reason about.
package collector

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/seance/connector"
)

// SystemState holds everything we know about the system right now.
type SystemState struct {
	Timestamp  time.Time              `json:"timestamp"`
	Snapshots  []connector.Snapshot   `json:"snapshots"`
	CollectMs  int                    `json:"collect_ms"`
	SourceCount int                   `json:"source_count"`
	ErrorCount int                    `json:"error_count"`
}

// Collector manages data sources and fetches from all of them.
type Collector struct {
	mu      sync.RWMutex
	sources []connector.Source
	timeout time.Duration
}

// New creates a collector.
func New(timeout time.Duration) *Collector {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Collector{timeout: timeout}
}

// Add registers a data source.
func (c *Collector) Add(src connector.Source) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sources = append(c.sources, src)
}

// SourceCount returns the number of registered sources.
func (c *Collector) SourceCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sources)
}

// Sources returns names of all registered sources.
func (c *Collector) Sources() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var names []string
	for _, s := range c.sources {
		names = append(names, s.Name()+" ("+s.Type()+")")
	}
	return names
}

// Gather queries all sources in parallel and returns unified system state.
func (c *Collector) Gather(ctx context.Context, question string) *SystemState {
	start := time.Now()
	fetchCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Extract keywords from the question to help sources filter
	hints := extractHints(question)

	c.mu.RLock()
	sources := make([]connector.Source, len(c.sources))
	copy(sources, c.sources)
	c.mu.RUnlock()

	var (
		snapshots []connector.Snapshot
		mu        sync.Mutex
		wg        sync.WaitGroup
		errors    int
	)

	for _, src := range sources {
		wg.Add(1)
		go func(s connector.Source) {
			defer wg.Done()
			snap, err := s.Fetch(fetchCtx, hints)
			if err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
			}
			if snap != nil {
				mu.Lock()
				snapshots = append(snapshots, *snap)
				mu.Unlock()
			}
		}(src)
	}

	wg.Wait()

	return &SystemState{
		Timestamp:   time.Now(),
		Snapshots:   snapshots,
		CollectMs:   int(time.Since(start).Milliseconds()),
		SourceCount: len(sources),
		ErrorCount:  errors,
	}
}

// BuildContext assembles all snapshots into a single text context for the LLM.
func BuildContext(state *SystemState) string {
	var sb strings.Builder

	sb.WriteString("LIVE SYSTEM STATE (collected ")
	sb.WriteString(state.Timestamp.Format("15:04:05"))
	sb.WriteString(", ")
	sb.WriteString(time.Now().Format("2006-01-02"))
	sb.WriteString(")\n")
	sb.WriteString(strings.Repeat("═", 60))
	sb.WriteString("\n\n")

	for _, snap := range state.Snapshots {
		sb.WriteString("─── ")
		sb.WriteString(strings.ToUpper(snap.Source))
		sb.WriteString(" (")
		sb.WriteString(snap.Type)
		sb.WriteString(", ")
		sb.WriteString(time.Now().Format("15:04:05"))
		sb.WriteString(") ")
		sb.WriteString(strings.Repeat("─", 40-len(snap.Source)))
		sb.WriteString("\n")

		if snap.Error != "" {
			sb.WriteString("[ERROR: ")
			sb.WriteString(snap.Error)
			sb.WriteString("]\n")
		}

		data := snap.Data
		// Truncate very large snapshots
		if len(data) > 8000 {
			data = data[:8000] + "\n...[truncated]"
		}
		sb.WriteString(data)
		sb.WriteString("\n\n")
	}

	sb.WriteString(strings.Repeat("═", 60))
	sb.WriteString("\n")

	return sb.String()
}

// extractHints pulls keywords from a question for source filtering.
func extractHints(question string) []string {
	// Simple keyword extraction — skip common words
	stopWords := map[string]bool{
		"why": true, "is": true, "the": true, "a": true, "an": true,
		"what": true, "how": true, "when": true, "where": true, "who": true,
		"are": true, "was": true, "were": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"can": true, "may": true, "might": true, "shall": true, "must": true,
		"my": true, "your": true, "our": true, "their": true, "its": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"with": true, "from": true, "of": true, "about": true, "into": true,
		"not": true, "so": true, "up": true, "out": true, "if": true,
		"and": true, "or": true, "but": true, "than": true, "then": true,
		"right": true, "now": true, "just": true, "very": true,
	}

	words := strings.Fields(strings.ToLower(question))
	var hints []string
	for _, w := range words {
		w = strings.Trim(w, "?.!,;:'\"")
		if len(w) >= 3 && !stopWords[w] {
			hints = append(hints, w)
		}
	}
	return hints
}
