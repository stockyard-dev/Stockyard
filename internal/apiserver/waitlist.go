package apiserver

import (
	"encoding/json"
	"log"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// waitlistRateLimiter tracks request counts per IP for the waitlist endpoint.
type waitlistRateLimiter struct {
	mu      sync.Mutex
	counts  map[string][]time.Time
	limit   int
	window  time.Duration
}

func newWaitlistRateLimiter() *waitlistRateLimiter {
	return &waitlistRateLimiter{
		counts: make(map[string][]time.Time),
		limit:  3,
		window: time.Minute,
	}
}

var waitlistLimiter = newWaitlistRateLimiter()

func (rl *waitlistRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	times := rl.counts[ip]
	// prune old entries
	fresh := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	rl.counts[ip] = fresh
	if len(fresh) >= rl.limit {
		return false
	}
	rl.counts[ip] = append(rl.counts[ip], now)
	return true
}

func clientIPForWaitlist(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		parts := strings.Split(v, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) handleWaitlist(w http.ResponseWriter, r *http.Request) {
	ip := clientIPForWaitlist(r)
	if !waitlistLimiter.allow(ip) {
		writeErr(w, http.StatusTooManyRequests, "rate limit exceeded — try again in a minute")
		return
	}

	var req struct {
		Email string `json:"email"`
		Tool  string `json:"tool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Tool = strings.TrimSpace(req.Tool)

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeErr(w, http.StatusBadRequest, "valid email required")
		return
	}
	if len(req.Email) > 320 {
		writeErr(w, http.StatusBadRequest, "email too long")
		return
	}
	if len(req.Tool) > 100 {
		req.Tool = req.Tool[:100]
	}

	_, err := s.db.conn.Exec(
		`INSERT OR IGNORE INTO waitlist (email, tool, ip) VALUES (?, ?, ?)`,
		req.Email, req.Tool, ip,
	)
	if err != nil {
		log.Printf("[waitlist] db error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to join waitlist")
		return
	}

	log.Printf("[waitlist] signup: email=%s tool=%s ip=%s", req.Email, req.Tool, ip)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"message":"You're on the waitlist for %s. We'll email you when it ships."}`, req.Tool)
}
