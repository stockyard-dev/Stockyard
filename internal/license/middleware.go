package license

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Middleware returns a proxy middleware that enforces license limits on every request.
// This should be the FIRST middleware in the chain (before IPFence, AuthGate, etc).
func (e *Enforcer) Middleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if err := e.Check(); err != nil {
				log.Printf("license: blocked request: %v", err)
				return nil, &LicenseError{
					Tier:    string(e.Tier()),
					Message: err.Error(),
				}
			}
			return next(ctx, req)
		}
	}
}

// LicenseError is returned when a request is blocked by license enforcement.
// The proxy API layer can detect this and return a 402 Payment Required.
type LicenseError struct {
	Tier    string
	Message string
}

func (e *LicenseError) Error() string {
	return fmt.Sprintf("license (%s): %s", e.Tier, e.Message)
}

// IsLicenseError returns true if the error is a LicenseError.
func IsLicenseError(err error) bool {
	_, ok := err.(*LicenseError)
	return ok
}

// UpgradeNudge tracks how often we should nudge free users to upgrade.
// Shows a gentle message in response headers periodically.
type UpgradeNudge struct {
	enforcer     *Enforcer
	requestCount atomic.Int64
}

// NewUpgradeNudge creates an upgrade nudge tracker.
func NewUpgradeNudge(e *Enforcer) *UpgradeNudge {
	return &UpgradeNudge{enforcer: e}
}

// ShouldNudge returns true + a message if it's time to show an upgrade prompt.
// Nudges on every 100th request for free tier, every 500th for starter.
func (u *UpgradeNudge) ShouldNudge() (bool, string) {
	count := u.requestCount.Add(1)
	tier := u.enforcer.Tier()

	switch tier {
	case TierFree:
		if count%100 == 0 {
			remaining := u.enforcer.limits.MaxRequestsPerDay - u.enforcer.dayCount.Load()
			return true, fmt.Sprintf("Free tier: %d requests remaining today. Upgrade at stockyard.dev/pricing", remaining)
		}
	case TierStarter:
		if count%500 == 0 {
			return true, "Unlock unlimited requests + exports with Pro. stockyard.dev/pricing"
		}
	}
	return false, ""
}
