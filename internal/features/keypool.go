package features

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// PooledKey tracks usage and health for a single API key.
type PooledKey struct {
	Name        string
	Key         string
	Provider    string
	Weight      int
	Requests    atomic.Int64
	Tokens      atomic.Int64
	Errors      atomic.Int64
	LastUsed    atomic.Int64 // unix nano
	CoolingDown atomic.Bool
	CooldownEnd atomic.Int64 // unix nano
}

// KeyPoolManager manages a pool of API keys with rotation strategies.
type KeyPoolManager struct {
	mu       sync.RWMutex
	keys     []*PooledKey
	strategy string
	cooldown time.Duration
	robin    atomic.Uint64 // round-robin counter
}

// NewKeyPool creates a new key pool from config.
func NewKeyPool(cfg config.KeyPoolConfig) *KeyPoolManager {
	pool := &KeyPoolManager{
		strategy: cfg.Strategy,
		cooldown: cfg.Cooldown.Duration,
	}

	if pool.strategy == "" {
		pool.strategy = "round-robin"
	}
	if pool.cooldown == 0 {
		pool.cooldown = 60 * time.Second
	}

	for _, entry := range cfg.Keys {
		if entry.Key == "" || isEnvTemplate(entry.Key) {
			continue
		}
		weight := entry.Weight
		if weight <= 0 {
			weight = 1
		}
		pk := &PooledKey{
			Name:     entry.Name,
			Key:      entry.Key,
			Provider: entry.Provider,
			Weight:   weight,
		}
		pool.keys = append(pool.keys, pk)
	}

	return pool
}

// Select picks the next key based on the configured strategy.
// Returns nil if no healthy keys are available.
func (kp *KeyPoolManager) Select() *PooledKey {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	healthy := kp.healthyKeys()
	if len(healthy) == 0 {
		return nil
	}

	switch kp.strategy {
	case "least-used":
		return kp.selectLeastUsed(healthy)
	case "random":
		return kp.selectRandom(healthy)
	default: // round-robin
		return kp.selectRoundRobin(healthy)
	}
}

// MarkError records an error for a key and triggers cooldown on 429.
func (kp *KeyPoolManager) MarkError(key *PooledKey, statusCode int) {
	key.Errors.Add(1)
	if statusCode == 429 {
		key.CoolingDown.Store(true)
		key.CooldownEnd.Store(time.Now().Add(kp.cooldown).UnixNano())
		log.Printf("keypool: key %q cooling down for %s (429 rate limit)", key.Name, kp.cooldown)
	}
}

// MarkSuccess records successful use of a key.
func (kp *KeyPoolManager) MarkSuccess(key *PooledKey, tokens int) {
	key.Requests.Add(1)
	key.Tokens.Add(int64(tokens))
	key.LastUsed.Store(time.Now().UnixNano())
}

// Stats returns current pool statistics.
func (kp *KeyPoolManager) Stats() []map[string]any {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	var stats []map[string]any
	for _, k := range kp.keys {
		cooling := k.CoolingDown.Load()
		if cooling && time.Now().UnixNano() > k.CooldownEnd.Load() {
			k.CoolingDown.Store(false)
			cooling = false
		}
		stats = append(stats, map[string]any{
			"name":        k.Name,
			"provider":    k.Provider,
			"weight":      k.Weight,
			"requests":    k.Requests.Load(),
			"tokens":      k.Tokens.Load(),
			"errors":      k.Errors.Load(),
			"cooling_down": cooling,
		})
	}
	return stats
}

// KeyCount returns number of keys in the pool.
func (kp *KeyPoolManager) KeyCount() int {
	kp.mu.RLock()
	defer kp.mu.RUnlock()
	return len(kp.keys)
}

func (kp *KeyPoolManager) healthyKeys() []*PooledKey {
	now := time.Now().UnixNano()
	var healthy []*PooledKey
	for _, k := range kp.keys {
		if k.CoolingDown.Load() {
			if now > k.CooldownEnd.Load() {
				k.CoolingDown.Store(false)
			} else {
				continue
			}
		}
		healthy = append(healthy, k)
	}
	return healthy
}

func (kp *KeyPoolManager) selectRoundRobin(keys []*PooledKey) *PooledKey {
	// Weight-aware: build expanded slice
	var expanded []*PooledKey
	for _, k := range keys {
		for range k.Weight {
			expanded = append(expanded, k)
		}
	}
	if len(expanded) == 0 {
		return keys[0]
	}
	idx := kp.robin.Add(1) - 1
	return expanded[idx%uint64(len(expanded))]
}

func (kp *KeyPoolManager) selectLeastUsed(keys []*PooledKey) *PooledKey {
	best := keys[0]
	bestReqs := best.Requests.Load()
	for _, k := range keys[1:] {
		reqs := k.Requests.Load()
		if reqs < bestReqs {
			best = k
			bestReqs = reqs
		}
	}
	return best
}

func (kp *KeyPoolManager) selectRandom(keys []*PooledKey) *PooledKey {
	// Weight-aware random
	totalWeight := 0
	for _, k := range keys {
		totalWeight += k.Weight
	}
	r := rand.Intn(totalWeight)
	for _, k := range keys {
		r -= k.Weight
		if r < 0 {
			return k
		}
	}
	return keys[0]
}

func isEnvTemplate(s string) bool {
	return len(s) > 3 && s[0] == '$' && s[1] == '{'
}

// KeyPoolMiddleware returns middleware that rotates API keys on outbound requests.
func KeyPoolMiddleware(pool *KeyPoolManager) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			key := pool.Select()
			if key == nil {
				return nil, fmt.Errorf("keypool: no healthy keys available (all cooling down)")
			}

			// Inject the selected key into the request's Extra map
			// The provider adapter reads this to override the configured API key.
			if req.Extra == nil {
				req.Extra = make(map[string]any)
			}
			req.Extra["_pool_key"] = key.Key
			req.Extra["_pool_key_name"] = key.Name
			if key.Provider != "" {
				req.Extra["_pool_provider"] = key.Provider
			}

			resp, err := next(ctx, req)
			if err != nil {
				// Check for 429 or other retryable errors
				if apiErr, ok := err.(*provider.ProviderAPIError); ok {
					pool.MarkError(key, apiErr.StatusCode)
					// If 429, try the next key automatically
					if apiErr.StatusCode == 429 {
						retryKey := pool.Select()
						if retryKey != nil {
							req.Extra["_pool_key"] = retryKey.Key
							req.Extra["_pool_key_name"] = retryKey.Name
							return next(ctx, req)
						}
					}
				}
				return nil, err
			}

			pool.MarkSuccess(key, resp.Usage.TotalTokens)
			return resp, nil
		}
	}
}
