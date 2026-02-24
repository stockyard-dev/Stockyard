package features

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// RateLimitConfig defines rate limiting settings.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
	RequestsPerHour   int
	Burst             int
	PerIP             bool
	PerUser           bool
	AbuseDetection    bool
	DuplicateThreshold int
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket.
func NewTokenBucket(maxTokens float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow attempts to consume one token. Returns true if allowed.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiter manages per-key rate limiters.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*TokenBucket
	config  RateLimitConfig
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		config:  cfg,
	}
}

// Allow checks if a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		maxTokens := float64(rl.config.Burst)
		if maxTokens == 0 {
			maxTokens = 10
		}
		refillRate := float64(rl.config.RequestsPerMinute) / 60.0
		bucket = NewTokenBucket(maxTokens, refillRate)
		rl.buckets[key] = bucket
	}
	return bucket.Allow()
}

// RateLimitMiddleware returns middleware that enforces rate limits.
func RateLimitMiddleware(limiter *RateLimiter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Use project as default key; could use IP or UserID
			key := req.Project
			if req.UserID != "" {
				key = req.UserID
			}

			if !limiter.Allow(key) {
				return nil, fmt.Errorf("rate limit exceeded for %s", key)
			}

			return next(ctx, req)
		}
	}
}
