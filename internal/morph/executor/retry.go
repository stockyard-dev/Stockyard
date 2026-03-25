package executor

import (
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RetryConfig controls retry behaviour for failed requests.
type RetryConfig struct {
	MaxRetries        int           // maximum number of retry attempts (default 3)
	InitialDelay      time.Duration // first backoff delay (default 1s)
	MaxDelay          time.Duration // cap on backoff delay (default 30s)
	RetryableStatuses map[int]bool  // HTTP status codes worth retrying
}

// DefaultRetryConfig returns a sensible default configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		RetryableStatuses: map[int]bool{
			429: true,
			500: true,
			502: true,
			503: true,
			504: true,
		},
	}
}

// rateLimitInfo tracks per-service rate limit state.
type rateLimitInfo struct {
	Remaining int
	ResetAt   time.Time
}

// rateLimits tracks rate limits per service.
type rateLimits struct {
	mu     sync.RWMutex
	limits map[string]*rateLimitInfo
}

func newRateLimits() *rateLimits {
	return &rateLimits{limits: map[string]*rateLimitInfo{}}
}

// update records rate limit info from response headers.
func (rl *rateLimits) update(service string, resp *http.Response) {
	if resp == nil {
		return
	}
	info := &rateLimitInfo{}
	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			info.Remaining = n
		}
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.ResetAt = time.Unix(ts, 0)
		}
	}
	rl.mu.Lock()
	rl.limits[service] = info
	rl.mu.Unlock()
}

// get retrieves rate limit info for a service.
func (rl *rateLimits) get(service string) *rateLimitInfo {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.limits[service]
}

// executeWithRetry sends the request and retries on transient failures with
// exponential backoff and jitter.
func executeWithRetry(client *http.Client, req *http.Request, cfg *RetryConfig, service string, rl *rateLimits) (*http.Response, error) {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Clone the request for each attempt (body must be re-readable, handled by caller via GetBody).
		var tryReq *http.Request
		if attempt == 0 {
			tryReq = req
		} else {
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				tryReq = req.Clone(req.Context())
				tryReq.Body = body
			} else {
				tryReq = req.Clone(req.Context())
			}
		}

		resp, err := client.Do(tryReq)
		if err != nil {
			lastErr = err
			if attempt < cfg.MaxRetries {
				sleepWithBackoff(attempt, cfg)
				continue
			}
			return nil, lastErr
		}

		// Track rate limits
		if rl != nil {
			rl.update(service, resp)
		}

		// Check if we should retry this status
		if !cfg.RetryableStatuses[resp.StatusCode] || attempt >= cfg.MaxRetries {
			return resp, nil
		}

		// For 429, respect Retry-After header
		if resp.StatusCode == 429 {
			if delay := parseRetryAfter(resp); delay > 0 {
				resp.Body.Close()
				select {
				case <-time.After(delay):
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
				lastErr = nil
				continue
			}
		}

		// Close body before retry
		resp.Body.Close()
		sleepWithBackoff(attempt, cfg)
	}

	return nil, lastErr
}

// sleepWithBackoff sleeps for an exponentially increasing duration with jitter.
func sleepWithBackoff(attempt int, cfg *RetryConfig) {
	delay := float64(cfg.InitialDelay) * math.Pow(2, float64(attempt))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	// Add jitter: +-25%
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	time.Sleep(time.Duration(delay + jitter))
}

// parseRetryAfter extracts a delay from the Retry-After header.
// Supports both seconds (integer) and HTTP-date formats.
func parseRetryAfter(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}
	// Try as seconds first
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	// Try as HTTP date
	if t, err := http.ParseTime(val); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}
	return 0
}
