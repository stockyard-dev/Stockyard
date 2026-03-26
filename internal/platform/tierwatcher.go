// Package platform — TierWatcher polls the database for tier changes
// so Stripe webhook upgrades take effect without redeploy.
package platform

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// TierWatcher periodically reads the active tier from platform_tier_state.
// All reads are lock-free on the hot path (RLock). Writes happen only
// when the background poller detects a change.
type TierWatcher struct {
	mu   sync.RWMutex
	tier Tier
	db   *sql.DB
	stop chan struct{}
}

// NewTierWatcher creates a watcher seeded with the boot-time tier.
// Call Start() to begin polling.
func NewTierWatcher(db *sql.DB, bootTier Tier) *TierWatcher {
	tw := &TierWatcher{
		tier: bootTier,
		db:   db,
		stop: make(chan struct{}),
	}
	// Immediately try to read from DB in case a tier was persisted before restart
	if dbTier, ok := tw.readFromDB(); ok && dbTier != bootTier {
		// DB tier wins over env-based boot tier UNLESS boot tier is Dev
		if bootTier != TierDev {
			tw.tier = dbTier
			log.Printf("[tierwatcher] boot tier overridden by DB: %s → %s", TierName(bootTier), TierName(dbTier))
		}
	}
	return tw
}

// CurrentTier returns the active tier. Thread-safe, called on every request.
func (tw *TierWatcher) CurrentTier() Tier {
	tw.mu.RLock()
	t := tw.tier
	tw.mu.RUnlock()
	return t
}

// Start begins the background poll loop. Call once from engine.Boot().
func (tw *TierWatcher) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tw.poll()
			case <-tw.stop:
				return
			}
		}
	}()
	log.Printf("[tierwatcher] polling every %s", interval)
}

// Stop terminates the background poller.
func (tw *TierWatcher) Stop() {
	close(tw.stop)
}

// Refresh forces an immediate re-read from DB. Call from webhook handler
// for instant tier activation instead of waiting for the next poll.
func (tw *TierWatcher) Refresh() {
	tw.poll()
}

func (tw *TierWatcher) poll() {
	newTier, ok := tw.readFromDB()
	if !ok {
		return
	}
	tw.mu.RLock()
	current := tw.tier
	tw.mu.RUnlock()

	// Dev mode is sticky — never downgrade from Dev via DB
	if current == TierDev {
		return
	}

	if newTier != current {
		tw.mu.Lock()
		tw.tier = newTier
		tw.mu.Unlock()
		log.Printf("[tierwatcher] tier changed: %s → %s", TierName(current), TierName(newTier))
	}
}

func (tw *TierWatcher) readFromDB() (Tier, bool) {
	var tierNum int
	err := tw.db.QueryRow("SELECT tier FROM platform_tier_state WHERE id = 'active'").Scan(&tierNum)
	if err != nil {
		return TierCommunity, false
	}
	return Tier(tierNum), true
}

// TierWrap wraps a middleware so it only executes when the live tier
// meets or exceeds the required minimum. When the tier is too low,
// the request passes straight through — zero overhead.
func TierWrap(name string, minTier Tier, tw *TierWatcher, mw proxy.Middleware) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		inner := mw(next)
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if tw.CurrentTier() < minTier {
				return next(ctx, req)
			}
			return inner(ctx, req)
		}
	}
}
