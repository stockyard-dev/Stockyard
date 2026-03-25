package features

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/auth"
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
	name            string // provider name, for event reporting
	failures        int
	threshold       int
	state           string // "closed", "open", "half-open"
	lastFailure     time.Time
	recoveryTimeout time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, recovery time.Duration) *CircuitBreaker {
	return NewCircuitBreakerNamed("", threshold, recovery)
}

// NewCircuitBreakerNamed creates a circuit breaker with a provider name for event reporting.
func NewCircuitBreakerNamed(name string, threshold int, recovery time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
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
			// Transition to half-open: allow probe requests through
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		// In half-open, allow requests through as probes. The next
		// RecordSuccess or RecordFailure determines the transition.
		return true
	}
	return true
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	prevState := cb.state
	cb.failures = 0
	cb.state = "closed"
	cb.mu.Unlock()

	if prevState != "closed" && cb.name != "" {
		fireWebhook("provider.health_changed", map[string]any{
			"provider":   cb.name,
			"state":      "closed",
			"prev_state": prevState,
		})
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	prevState := cb.state
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == "half-open" || cb.failures >= cb.threshold {
		cb.state = "open"
	}
	newState := cb.state
	cb.mu.Unlock()

	if prevState != newState && cb.name != "" {
		fireWebhook("provider.health_changed", map[string]any{
			"provider":   cb.name,
			"state":      newState,
			"prev_state": prevState,
			"failures":   cb.failures,
		})
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Failures returns the current failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// Reset forces the circuit breaker back to closed state, clearing failures.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	prevState := cb.state
	cb.failures = 0
	cb.state = "closed"
	cb.lastFailure = time.Time{}
	cb.mu.Unlock()

	if prevState != "closed" && cb.name != "" {
		fireWebhook("provider.health_changed", map[string]any{
			"provider":   cb.name,
			"state":      "closed",
			"prev_state": prevState,
			"reset":      true,
		})
	}
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
		breakers[p] = NewCircuitBreakerNamed(p, cfg.FailureThreshold, cfg.RecoveryTimeout)
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

// ResetBreaker resets a single provider's circuit breaker.
func (fr *FailoverRouter) ResetBreaker(name string) bool {
	cb, ok := fr.breakers[name]
	if !ok {
		return false
	}
	cb.Reset()
	log.Printf("failover: circuit breaker for %s manually reset", name)
	return true
}

// ResetAll resets all circuit breakers.
func (fr *FailoverRouter) ResetAll() {
	for name, cb := range fr.breakers {
		cb.Reset()
		log.Printf("failover: circuit breaker for %s manually reset", name)
	}
}

// BreakerStates returns the current state of all circuit breakers.
func (fr *FailoverRouter) BreakerStates() map[string]map[string]any {
	states := make(map[string]map[string]any, len(fr.breakers))
	for name, cb := range fr.breakers {
		states[name] = map[string]any{
			"state":    cb.State(),
			"failures": cb.Failures(),
		}
	}
	return states
}

// isRetryableError checks if an error should trigger failover to the next provider.
func isRetryableError(err error) bool {
	if apiErr, ok := err.(*provider.ProviderAPIError); ok {
		return apiErr.IsRetryable()
	}
	// Network errors, timeouts, etc. are always retryable
	return true
}

// isModelMismatchError checks if the error indicates the provider doesn't
// support the requested model (e.g. sending a Claude model to OpenAI).
// These errors should skip to the next provider WITHOUT penalizing the
// circuit breaker — the provider is healthy, just wrong for this model.
func isModelMismatchError(err error) bool {
	apiErr, ok := err.(*provider.ProviderAPIError)
	if !ok {
		return false
	}
	// 404 = model not found, the primary signal for model mismatch.
	// Some providers also return 400 with "model not found" style messages,
	// but 404 is the reliable indicator across OpenAI, Anthropic, and Groq.
	return apiErr.StatusCode == 404
}

// modelAwareProviderOrder reorders the failover chain so the natural provider
// for the requested model goes first, followed by remaining configured providers.
// This prevents wasting attempts on providers that can't handle the model.
func modelAwareProviderOrder(model string, configuredProviders []string) []string {
	natural := provider.ProviderForModel(model)

	// Build reordered list: natural provider first (if in the chain),
	// then the rest in their configured order.
	ordered := make([]string, 0, len(configuredProviders))
	for _, p := range configuredProviders {
		if p == natural {
			ordered = append([]string{p}, ordered...)
		} else {
			ordered = append(ordered, p)
		}
	}
	return ordered
}

// FailoverMiddleware returns middleware that tries providers in priority order.
// The provider order is model-aware: the natural provider for the requested model
// is tried first, followed by the remaining failover chain.
//
// Note: failover retries requests on alternate providers, which is safe for
// idempotent read-only completions. Streaming requests are handled separately
// via the streaming failover path in proxy/streaming.go, not this middleware.
func FailoverMiddleware(router *FailoverRouter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// BYOK: if the user provided their own API key, skip the failover
			// chain entirely and let the inner handler use the ephemeral provider.
			if autoP, _ := auth.AutoProviderFromContext(ctx); autoP != nil {
				return next(ctx, req)
			}

			// Streaming requests should not be retried through this middleware
			// as they are handled by the streaming failover path directly.
			if req.Stream {
				return next(ctx, req)
			}

			// Reorder providers so the natural one for this model goes first.
			providers := modelAwareProviderOrder(req.Model, router.config.Providers)

			var lastErr error
			attempted := 0

			for _, name := range providers {
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
				attempted++

				if err != nil {
					// Model mismatch (e.g. Claude model sent to OpenAI → 404):
					// skip to next provider, don't penalize the circuit breaker.
					if isModelMismatchError(err) {
						log.Printf("failover: %s does not support model %s, skipping", name, req.Model)
						continue
					}

					if !isRetryableError(err) {
						// Non-retryable (e.g. 400, 401) — provider is healthy,
						// the request itself is bad. Don't failover.
						cb.RecordSuccess()
						return nil, err
					}

					cb.RecordFailure()
					lastErr = err
					log.Printf("failover: %s failed (%v), trying next", name, err)

					// Check context cancellation before trying next provider
					if ctx.Err() != nil {
						return nil, fmt.Errorf("failover aborted: %w", ctx.Err())
					}
					continue
				}

				cb.RecordSuccess()
				resp.Provider = name
				return resp, nil
			}

			if attempted == 0 {
				return nil, fmt.Errorf("all providers unavailable (circuit breakers open)")
			}
			return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
		}
	}
}
