// Package detect monitors applications for errors and anomalies.
// When an error is detected, it captures the full context needed for diagnosis.
package detect

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Error represents a detected production error with full context.
type Error struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Type       string            `json:"type"`        // http_500, panic, timeout, anomaly
	Message    string            `json:"message"`
	Path       string            `json:"path"`
	Method     string            `json:"method"`
	StatusCode int               `json:"status_code"`
	RequestBody string           `json:"request_body,omitempty"`
	ResponseBody string          `json:"response_body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
	Count      int               `json:"count"` // how many times this exact error occurred
	FirstSeen  time.Time         `json:"first_seen"`
	LastSeen   time.Time         `json:"last_seen"`
}

// ErrorHandler is called when an error is detected.
type ErrorHandler func(err Error)

// Monitor watches for errors in HTTP traffic.
type Monitor struct {
	mu       sync.RWMutex
	errors   []Error
	seen     map[string]*Error // dedup by fingerprint
	handlers []ErrorHandler
	maxErrs  int
}

// New creates an error monitor.
func New() *Monitor {
	return &Monitor{
		seen:    map[string]*Error{},
		maxErrs: 1000,
	}
}

// OnError registers a handler called when new errors are detected.
func (m *Monitor) OnError(h ErrorHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, h)
}

// RecordError manually records an error.
func (m *Monitor) RecordError(e Error) {
	if e.ID == "" {
		e.ID = fmt.Sprintf("err-%d", time.Now().UnixNano())
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if e.FirstSeen.IsZero() {
		e.FirstSeen = e.Timestamp
	}
	e.LastSeen = e.Timestamp

	fp := fingerprint(e)

	m.mu.Lock()
	if existing, ok := m.seen[fp]; ok {
		existing.Count++
		existing.LastSeen = e.Timestamp
		m.mu.Unlock()
		return // not new, just increment count
	}

	e.Count = 1
	m.seen[fp] = &e
	m.errors = append(m.errors, e)
	if len(m.errors) > m.maxErrs {
		m.errors = m.errors[len(m.errors)-m.maxErrs:]
	}

	handlers := make([]ErrorHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.Unlock()

	// Notify handlers (outside lock)
	for _, h := range handlers {
		h(e)
	}
}

// Middleware returns HTTP middleware that detects errors automatically.
func (m *Monitor) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request body
		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(r.Body, 32768))
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Wrap response writer
		rw := &responseCapture{ResponseWriter: w, statusCode: 200}
		
		// Catch panics
		defer func() {
			if rec := recover(); rec != nil {
				m.RecordError(Error{
					Type:        "panic",
					Message:     fmt.Sprintf("panic: %v", rec),
					Path:        r.URL.Path,
					Method:      r.Method,
					RequestBody: truncate(string(reqBody), 2000),
				})
				http.Error(w, "Internal Server Error", 500)
			}
		}()

		next.ServeHTTP(rw, r)

		// Detect HTTP errors
		if rw.statusCode >= 500 {
			headers := map[string]string{}
			for k, vals := range r.Header {
				kl := strings.ToLower(k)
				if kl == "authorization" || kl == "cookie" {
					continue
				}
				headers[k] = strings.Join(vals, ", ")
			}

			m.RecordError(Error{
				Type:         "http_500",
				Message:      fmt.Sprintf("HTTP %d on %s %s", rw.statusCode, r.Method, r.URL.Path),
				Path:         r.URL.Path,
				Method:       r.Method,
				StatusCode:   rw.statusCode,
				RequestBody:  truncate(string(reqBody), 2000),
				ResponseBody: truncate(rw.body.String(), 2000),
				Headers:      headers,
			})
		}
	})
}

// RecentErrors returns the last N unique errors.
func (m *Monitor) RecentErrors(n int) []Error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n > len(m.errors) {
		n = len(m.errors)
	}
	out := make([]Error, n)
	copy(out, m.errors[len(m.errors)-n:])
	return out
}

// ErrorCount returns total unique errors detected.
func (m *Monitor) ErrorCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors)
}

type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *responseCapture) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseCapture) Write(b []byte) (int, error) {
	if r.statusCode >= 500 {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

func fingerprint(e Error) string {
	return fmt.Sprintf("%s:%s:%s:%d", e.Type, e.Method, e.Path, e.StatusCode)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
