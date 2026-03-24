package features

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// AdaptiveRateLimiter adjusts rate limits based on real-time provider health signals.
type AdaptiveRateLimiter struct {
	conn              *sql.DB
	mu                sync.RWMutex
	providerLimits    map[string]*providerLimit
	baselineLat       map[string]float64 // baseline latency per provider
	requestsThrottled atomic.Int64
	requestsAllowed   atomic.Int64
}

type providerLimit struct {
	maxConcurrent int
	current       atomic.Int32
	throttled     bool
	lastCheck     time.Time
}

// NewAdaptiveRateLimiter creates an adaptive rate limiter.
func NewAdaptiveRateLimiter(conn *sql.DB) *AdaptiveRateLimiter {
	arl := &AdaptiveRateLimiter{
		conn:           conn,
		providerLimits: make(map[string]*providerLimit),
		baselineLat:    make(map[string]float64),
	}
	arl.loadBaselines()
	go arl.monitorLoop()
	return arl
}

func (arl *AdaptiveRateLimiter) loadBaselines() {
	rows, err := arl.conn.Query(`
		SELECT provider, AVG(duration_ms) as avg_lat
		FROM observe_traces
		WHERE created_at >= datetime('now', '-24 hours') AND provider != '' AND status = 'ok'
		GROUP BY provider
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	arl.mu.Lock()
	defer arl.mu.Unlock()
	for rows.Next() {
		var prov string
		var avgLat float64
		rows.Scan(&prov, &avgLat)
		arl.baselineLat[prov] = avgLat
		if _, ok := arl.providerLimits[prov]; !ok {
			arl.providerLimits[prov] = &providerLimit{maxConcurrent: 50}
		}
	}
}

func (arl *AdaptiveRateLimiter) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		arl.adjustLimits()
	}
}

func (arl *AdaptiveRateLimiter) adjustLimits() {
	rows, err := arl.conn.Query(`
		SELECT provider,
			AVG(duration_ms) as current_lat,
			CAST(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS REAL) / MAX(COUNT(*), 1) as err_rate
		FROM observe_traces
		WHERE created_at >= datetime('now', '-5 minutes') AND provider != ''
		GROUP BY provider
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	arl.mu.Lock()
	defer arl.mu.Unlock()

	for rows.Next() {
		var prov string
		var currentLat, errRate float64
		rows.Scan(&prov, &currentLat, &errRate)

		baseline, ok := arl.baselineLat[prov]
		if !ok {
			baseline = currentLat
			arl.baselineLat[prov] = baseline
		}

		limit, ok := arl.providerLimits[prov]
		if !ok {
			limit = &providerLimit{maxConcurrent: 50}
			arl.providerLimits[prov] = limit
		}

		// If latency increased >50% above baseline, throttle.
		if baseline > 0 && currentLat > baseline*1.5 {
			limit.maxConcurrent = max(limit.maxConcurrent/2, 5)
			limit.throttled = true
			log.Printf("[adaptive-ratelimit] throttling %s: latency %.0fms > baseline %.0fms", prov, currentLat, baseline)
		} else if errRate > 0.1 {
			// If error rate >10%, reduce concurrency.
			limit.maxConcurrent = max(limit.maxConcurrent/2, 5)
			limit.throttled = true
			log.Printf("[adaptive-ratelimit] throttling %s: error rate %.1f%%", prov, errRate*100)
		} else if limit.throttled {
			// Recovery: gradually increase limits.
			limit.maxConcurrent = min(limit.maxConcurrent+5, 50)
			if limit.maxConcurrent >= 50 {
				limit.throttled = false
			}
		}
		limit.lastCheck = time.Now()
	}
}

// AdaptiveRateLimitMiddleware creates the middleware.
func AdaptiveRateLimitMiddleware(arl *AdaptiveRateLimiter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			prov := req.Provider
			if prov == "" {
				return next(ctx, req)
			}

			arl.mu.RLock()
			limit, ok := arl.providerLimits[prov]
			arl.mu.RUnlock()

			if ok && limit.throttled {
				current := limit.current.Load()
				if int(current) >= limit.maxConcurrent {
					arl.requestsThrottled.Add(1)
					return nil, fmt.Errorf("rate limited: provider %s throttled (concurrency %d/%d)", prov, current, limit.maxConcurrent)
				}
			}

			if ok {
				limit.current.Add(1)
				defer limit.current.Add(-1)
			}
			arl.requestsAllowed.Add(1)
			return next(ctx, req)
		}
	}
}

// Stats returns adaptive rate limiter stats.
func (arl *AdaptiveRateLimiter) Stats() map[string]any {
	arl.mu.RLock()
	defer arl.mu.RUnlock()

	providers := make(map[string]any)
	for prov, limit := range arl.providerLimits {
		providers[prov] = map[string]any{
			"max_concurrent": limit.maxConcurrent,
			"current":        limit.current.Load(),
			"throttled":      limit.throttled,
			"baseline_lat":   arl.baselineLat[prov],
		}
	}
	return map[string]any{
		"providers":       providers,
		"total_throttled": arl.requestsThrottled.Load(),
		"total_allowed":   arl.requestsAllowed.Load(),
	}
}
