package features

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// FailoverConfig defines failover routing settings.
type FailoverConfig struct {
	Enabled          bool
	Strategy         string   // "priority", "round-robin", "latency-based"
	Providers        []string // Ordered fallback chain
	FailureThreshold int
	RecoveryTimeout  time.Duration
}

// CircuitBreaker tracks provider health for failover decisions.
type CircuitBreaker struct {
	mu              sync.Mutex
	failures        int
	threshold       int
	state           string // "closed", "open", "half-open"
	lastFailure     time.Time
	recoveryTimeout time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, recovery time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:       threshold,
		state:           "closed",
		recoveryTimeout: recovery,
	}
}

// Allow returns true if requests should be allowed through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case "closed":
		return true
	case "open":
		if time.Since(cb.lastFailure) > cb.recoveryTimeout {
			// Transition to half-open: allow ONE probe request
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		// Already have a probe in flight, block additional requests
		return false
	}
	return true
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = "closed"
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == "half-open" || cb.failures >= cb.threshold {
		cb.state = "open"
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// FailoverRouter manages provider failover with circuit breakers.
type FailoverRouter struct {
	config   FailoverConfig
	breakers map[string]*CircuitBreaker
	senders  map[string]proxy.Handler
}

// NewFailoverRouter creates a new failover router.
func NewFailoverRouter(cfg FailoverConfig) *FailoverRouter {
	breakers := make(map[string]*CircuitBreaker)
	for _, p := range cfg.Providers {
		breakers[p] = NewCircuitBreaker(cfg.FailureThreshold, cfg.RecoveryTimeout)
	}
	return &FailoverRouter{
		config:   cfg,
		breakers: breakers,
		senders:  make(map[string]proxy.Handler),
	}
}

// RegisterSender registers a handler for a specific provider.
func (fr *FailoverRouter) RegisterSender(name string, handler proxy.Handler) {
	fr.senders[name] = handler
}

// isRetryableError checks if an error should trigger failover.
func isRetryableError(err error) bool {
	if apiErr, ok := err.(*provider.ProviderAPIError); ok {
		return apiErr.IsRetryable()
	}
	// Network errors, timeouts, etc. are always retryable
	return true
}

// FailoverMiddleware returns middleware that tries providers in priority order.
func FailoverMiddleware(router *FailoverRouter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			var lastErr error

			for _, name := range router.config.Providers {
				cb, ok := router.breakers[name]
				if !ok || !cb.Allow() {
					continue
				}

				sender, ok := router.senders[name]
				if !ok {
					sender = next // fallback to default handler
				}

				// Override the provider for this attempt
				origProvider := req.Provider
				req.Provider = name

				resp, err := sender(ctx, req)
				req.Provider = origProvider // restore

				if err != nil {
					if !isRetryableError(err) {
						// Non-retryable (e.g. 400, 401) — don't failover, return immediately
						cb.RecordSuccess() // provider is healthy, just bad request
						return nil, err
					}
					cb.RecordFailure()
					lastErr = err
					log.Printf("failover: %s failed (%v), trying next", name, err)
					continue
				}

				cb.RecordSuccess()
				resp.Provider = name
				return resp, nil
			}

			return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
		}
	}
}
