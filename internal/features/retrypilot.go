package features

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, reject requests
	CircuitHalfOpen                     // Testing recovery
)

// RetryPilotEngine implements intelligent retry with circuit breaking.
type RetryPilotEngine struct {
	cfg      config.RetryPilotConfig
	circuits sync.Map // endpoint/model key → *circuitBreaker

	totalReqs    atomic.Int64
	totalRetries atomic.Int64
	totalSuccess atomic.Int64
	totalFailed  atomic.Int64
	downgrades   atomic.Int64
	circuitTrips atomic.Int64

	// Rate limiting retries
	retryBudget struct {
		mu       sync.Mutex
		count    int
		windowStart time.Time
	}
}

type circuitBreaker struct {
	mu              sync.Mutex
	state           CircuitState
	failures        int
	successes       int // for half-open
	lastFailure     time.Time
	openedAt        time.Time
	threshold       int
	recoveryTimeout time.Duration
	halfOpenMax     int
}

// NewRetryPilot creates a new retry engine.
func NewRetryPilot(cfg config.RetryPilotConfig) *RetryPilotEngine {
	return &RetryPilotEngine{cfg: cfg}
}

// getCircuit returns (or creates) the circuit breaker for a model/endpoint.
func (rp *RetryPilotEngine) getCircuit(key string) *circuitBreaker {
	v, _ := rp.circuits.LoadOrStore(key, &circuitBreaker{
		state:           CircuitClosed,
		threshold:       rp.cfg.CircuitBreaker.FailureThreshold,
		recoveryTimeout: rp.cfg.CircuitBreaker.RecoveryTimeout.Duration,
		halfOpenMax:     rp.cfg.CircuitBreaker.HalfOpenRequests,
	})
	cb := v.(*circuitBreaker)
	if cb.threshold == 0 {
		cb.threshold = 5
	}
	if cb.recoveryTimeout == 0 {
		cb.recoveryTimeout = 60 * time.Second
	}
	if cb.halfOpenMax == 0 {
		cb.halfOpenMax = 2
	}
	return cb
}

// checkCircuit returns an error if the circuit is open.
func (cb *circuitBreaker) check() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitOpen:
		// Check if recovery timeout has elapsed
		if time.Since(cb.openedAt) > cb.recoveryTimeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			return nil // Allow probe request
		}
		return fmt.Errorf("circuit open since %v", cb.openedAt.Format(time.RFC3339))

	case CircuitHalfOpen:
		if cb.successes >= cb.halfOpenMax {
			// Already enough probes in flight
			return fmt.Errorf("circuit half-open, probing")
		}
		return nil

	default:
		return nil
	}
}

// recordSuccess records a successful request.
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.halfOpenMax {
			cb.state = CircuitClosed
		}
	}
}

// recordFailure records a failed request. Returns true if circuit just opened.
func (cb *circuitBreaker) recordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		return true
	}

	if cb.failures >= cb.threshold && cb.state == CircuitClosed {
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		return true
	}

	return false
}

// checkRetryBudget returns true if we haven't exceeded the per-minute retry budget.
func (rp *RetryPilotEngine) checkRetryBudget() bool {
	budget := rp.cfg.Budget.MaxPerMinute
	if budget == 0 {
		return true // No budget set
	}

	rp.retryBudget.mu.Lock()
	defer rp.retryBudget.mu.Unlock()

	now := time.Now()
	if now.Sub(rp.retryBudget.windowStart) > time.Minute {
		rp.retryBudget.count = 0
		rp.retryBudget.windowStart = now
	}

	if rp.retryBudget.count >= budget {
		return false
	}
	rp.retryBudget.count++
	return true
}

// calculateDelay returns the retry delay with backoff and jitter.
func (rp *RetryPilotEngine) calculateDelay(attempt int) time.Duration {
	baseDelay := rp.cfg.BaseDelay.Duration
	if baseDelay == 0 {
		baseDelay = 1 * time.Second
	}
	maxDelay := rp.cfg.MaxDelay.Duration
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}

	// Exponential backoff
	delay := float64(baseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Apply jitter
	jitter := rp.cfg.Jitter
	if jitter == "" {
		jitter = "full"
	}

	switch jitter {
	case "full":
		delay = rand.Float64() * delay
	case "equal":
		delay = delay/2 + rand.Float64()*(delay/2)
	case "none":
		// No jitter
	}

	return time.Duration(delay)
}

// getDowngradeModel returns the cheaper fallback model, if configured.
func (rp *RetryPilotEngine) getDowngradeModel(model string) (string, bool) {
	if !rp.cfg.Downgrade.Enabled {
		return "", false
	}
	for original, downgrade := range rp.cfg.Downgrade.DowngradeMap {
		if original == model {
			return downgrade, true
		}
	}
	return "", false
}

// Execute runs a request with intelligent retries.
func (rp *RetryPilotEngine) Execute(ctx context.Context, req *provider.Request, handler proxy.Handler) (*provider.Response, error) {
	rp.totalReqs.Add(1)

	maxRetries := rp.cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	currentModel := req.Model
	consecutiveFailures := 0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check circuit breaker
		cb := rp.getCircuit(currentModel)
		if err := cb.check(); err != nil {
			// Circuit is open — try downgrade immediately
			if downModel, ok := rp.getDowngradeModel(currentModel); ok {
				log.Printf("retrypilot: circuit open for %s, downgrading to %s", currentModel, downModel)
				currentModel = downModel
				rp.downgrades.Add(1)
				cb = rp.getCircuit(currentModel)
				if err := cb.check(); err != nil {
					return nil, fmt.Errorf("retrypilot: circuit open for downgrade model %s too", currentModel)
				}
			} else {
				return nil, fmt.Errorf("retrypilot: %w", err)
			}
		}

		// Deadline awareness — check if we have enough time for another attempt
		if attempt > 0 && rp.cfg.Deadline.Enabled {
			deadline, ok := ctx.Deadline()
			if ok {
				minRemaining := rp.cfg.Deadline.MinRemaining.Duration
				if minRemaining == 0 {
					minRemaining = 5 * time.Second
				}
				if time.Until(deadline) < minRemaining {
					log.Printf("retrypilot: aborting retry, only %v remaining (min: %v)",
						time.Until(deadline), minRemaining)
					break
				}
			}
		}

		// Retry budget check
		if attempt > 0 && !rp.checkRetryBudget() {
			log.Printf("retrypilot: retry budget exhausted")
			break
		}

		// Delay between retries
		if attempt > 0 {
			delay := rp.calculateDelay(attempt)
			rp.totalRetries.Add(1)
			log.Printf("retrypilot: attempt %d/%d for %s (delay: %v)", attempt+1, maxRetries+1, currentModel, delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Execute request
		reqCopy := *req
		reqCopy.Model = currentModel
		resp, err := handler(ctx, &reqCopy)

		if err == nil {
			cb.recordSuccess()
			rp.totalSuccess.Add(1)
			if attempt > 0 {
				log.Printf("retrypilot: succeeded on attempt %d model=%s", attempt+1, currentModel)
			}
			return resp, nil
		}

		// Check if error is retryable
		if !isRetryableErr(err) {
			rp.totalFailed.Add(1)
			return nil, err
		}

		// Record failure
		tripped := cb.recordFailure()
		if tripped {
			rp.circuitTrips.Add(1)
			log.Printf("retrypilot: circuit opened for %s after %d failures", currentModel, cb.threshold)
		}

		consecutiveFailures++

		// Model downgrade after N consecutive failures
		if rp.cfg.Downgrade.Enabled && consecutiveFailures >= rp.cfg.Downgrade.AfterFailures {
			if downModel, ok := rp.getDowngradeModel(currentModel); ok {
				log.Printf("retrypilot: downgrading %s → %s after %d failures",
					currentModel, downModel, consecutiveFailures)
				currentModel = downModel
				consecutiveFailures = 0
				rp.downgrades.Add(1)
			}
		}
	}

	rp.totalFailed.Add(1)
	return nil, fmt.Errorf("retrypilot: all %d attempts failed for %s", maxRetries+1, req.Model)
}

// isRetryableError checks if an error should trigger a retry.
func isRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	// Provider API errors with retryable status codes
	if apiErr, ok := err.(*provider.ProviderAPIError); ok {
		return apiErr.IsRetryable()
	}
	// Context errors are not retryable
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	// Default: assume retryable for network errors
	return true
}

// Stats returns retry engine statistics.
func (rp *RetryPilotEngine) Stats() map[string]any {
	circuits := make(map[string]string)
	rp.circuits.Range(func(key, value any) bool {
		model := key.(string)
		cb := value.(*circuitBreaker)
		cb.mu.Lock()
		state := "closed"
		switch cb.state {
		case CircuitOpen:
			state = "open"
		case CircuitHalfOpen:
			state = "half-open"
		}
		circuits[model] = state
		cb.mu.Unlock()
		return true
	})

	return map[string]any{
		"total_requests": rp.totalReqs.Load(),
		"total_retries":  rp.totalRetries.Load(),
		"total_success":  rp.totalSuccess.Load(),
		"total_failed":   rp.totalFailed.Load(),
		"downgrades":     rp.downgrades.Load(),
		"circuit_trips":  rp.circuitTrips.Load(),
		"circuits":       circuits,
	}
}

// RetryPilotMiddleware returns middleware that wraps requests with intelligent retries.
func RetryPilotMiddleware(rp *RetryPilotEngine) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return rp.Execute(ctx, req, next)
		}
	}
}
