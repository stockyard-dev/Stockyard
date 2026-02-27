package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// Broadcaster manages SSE connections and broadcasts events to all connected dashboards.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

// NewBroadcaster creates a new SSE broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan []byte]struct{}),
	}
}

// Send broadcasts an event to all connected clients.
// Accepts any JSON-serializable value. Implements features.EventBroadcaster.
func (b *Broadcaster) Send(event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("broadcast marshal error: %v", err)
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- data:
		default:
			// Client is slow, drop event
		}
	}
}

// ClientCount returns the number of connected SSE clients.
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// AddListener registers a Go callback that receives every broadcast event.
// Returns an unsubscribe function.
func (b *Broadcaster) AddListener(fn func([]byte)) func() {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		for data := range ch {
			fn(data)
		}
	}()

	return func() {
		b.mu.Lock()
		delete(b.clients, ch)
		close(ch)
		b.mu.Unlock()
	}
}

// RegisterSSE mounts the SSE endpoint on the given ServeMux.
func (b *Broadcaster) RegisterSSE(mux *http.ServeMux) {
	mux.HandleFunc("GET /ui/events", b.handleSSE)
}

func (b *Broadcaster) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan []byte, 32)

	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
	}()

	// Send initial connected event
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
