package license

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TierLimits defines the operational limits for a pricing tier.
type TierLimits struct {
	MaxRequestsPerDay   int64 // 0 = unlimited
	MaxRequestsPerMonth int64 // 0 = unlimited
	MaxProductsEnabled  int   // suite only: 0 = unlimited
	MultiInstance       bool  // can run multiple instances
	WhiteLabel          bool  // can customize branding
	PrioritySupport     bool
	DashboardAccess     bool
	APIAccess           bool
	ExportAccess        bool // training export, compliance export
}

// Limits returns the TierLimits for a given tier.
func Limits(tier Tier) TierLimits {
	switch tier {
	case TierStarter:
		return TierLimits{
			MaxRequestsPerDay:   10_000,
			MaxRequestsPerMonth: 250_000,
			MaxProductsEnabled:  0, // unlimited for individual; suite starter = all
			DashboardAccess:     true,
			APIAccess:           true,
		}
	case TierPro:
		return TierLimits{
			MaxRequestsPerDay:   0, // unlimited
			MaxRequestsPerMonth: 0,
			MaxProductsEnabled:  0,
			DashboardAccess:     true,
			APIAccess:           true,
			ExportAccess:        true,
		}
	case TierTeam:
		return TierLimits{
			MaxRequestsPerDay:   0,
			MaxRequestsPerMonth: 0,
			MaxProductsEnabled:  0,
			MultiInstance:        true,
			WhiteLabel:           true,
			PrioritySupport:     true,
			DashboardAccess:     true,
			APIAccess:           true,
			ExportAccess:        true,
		}
	case TierEnterprise:
		return TierLimits{
			MaxRequestsPerDay:   0,
			MaxRequestsPerMonth: 0,
			MaxProductsEnabled:  0,
			MultiInstance:        true,
			WhiteLabel:           true,
			PrioritySupport:     true,
			DashboardAccess:     true,
			APIAccess:           true,
			ExportAccess:        true,
		}
	default: // TierFree
		return TierLimits{
			MaxRequestsPerDay:   1_000,
			MaxRequestsPerMonth: 10_000,
			MaxProductsEnabled:  5, // suite only
			DashboardAccess:     true,
			APIAccess:           true,
		}
	}
}

// Enforcer tracks usage and enforces tier limits at runtime.
type Enforcer struct {
	license *License
	limits  TierLimits

	mu       sync.Mutex
	dayStart time.Time
	moStart  time.Time

	dayCount   atomic.Int64
	monthCount atomic.Int64
	totalCount atomic.Int64

	blocked atomic.Int64
}

// NewEnforcer creates an Enforcer for the given license.
func NewEnforcer(lic *License) *Enforcer {
	now := time.Now()
	return &Enforcer{
		license:  lic,
		limits:   Limits(lic.Payload.Tier),
		dayStart: startOfDay(now),
		moStart:  startOfMonth(now),
	}
}

// Check determines if a request is allowed under the current tier limits.
// Returns nil if allowed, or an error describing why the request was blocked.
func (e *Enforcer) Check() error {
	// Expired license = free tier limits
	if e.license.IsExpired() {
		return fmt.Errorf("license expired on %s — operating in free tier", e.license.ExpiresAt.Format("2006-01-02"))
	}

	now := time.Now()

	// Reset counters on day/month rollover
	e.mu.Lock()
	if now.After(e.dayStart.Add(24 * time.Hour)) {
		e.dayCount.Store(0)
		e.dayStart = startOfDay(now)
	}
	if now.After(e.moStart.AddDate(0, 1, 0)) {
		e.monthCount.Store(0)
		e.moStart = startOfMonth(now)
	}
	e.mu.Unlock()

	// Check daily limit
	if e.limits.MaxRequestsPerDay > 0 {
		current := e.dayCount.Load()
		if current >= e.limits.MaxRequestsPerDay {
			e.blocked.Add(1)
			return fmt.Errorf("daily request limit reached (%d/%d) — upgrade at stockyard.dev/pricing",
				current, e.limits.MaxRequestsPerDay)
		}
	}

	// Check monthly limit
	if e.limits.MaxRequestsPerMonth > 0 {
		current := e.monthCount.Load()
		if current >= e.limits.MaxRequestsPerMonth {
			e.blocked.Add(1)
			return fmt.Errorf("monthly request limit reached (%d/%d) — upgrade at stockyard.dev/pricing",
				current, e.limits.MaxRequestsPerMonth)
		}
	}

	// Allowed — increment counters
	e.dayCount.Add(1)
	e.monthCount.Add(1)
	e.totalCount.Add(1)

	return nil
}

// Stats returns usage statistics for dashboard display.
func (e *Enforcer) Stats() map[string]any {
	lim := e.limits
	stats := map[string]any{
		"tier":             string(e.license.Payload.Tier),
		"product":          e.license.Payload.Product,
		"customer_id":      e.license.Payload.CustomerID,
		"valid":            e.license.Valid,
		"requests_today":   e.dayCount.Load(),
		"requests_month":   e.monthCount.Load(),
		"requests_total":   e.totalCount.Load(),
		"requests_blocked": e.blocked.Load(),
		"daily_limit":      lim.MaxRequestsPerDay,
		"monthly_limit":    lim.MaxRequestsPerMonth,
		"whitelabel":       lim.WhiteLabel,
		"multi_instance":   lim.MultiInstance,
		"export_access":    lim.ExportAccess,
	}
	if !e.license.ExpiresAt.IsZero() {
		stats["expires_at"] = e.license.ExpiresAt.Format(time.RFC3339)
		stats["days_remaining"] = int(time.Until(e.license.ExpiresAt).Hours() / 24)
	}
	return stats
}

// Tier returns the current effective tier.
func (e *Enforcer) Tier() Tier {
	if e.license.IsExpired() {
		return TierFree
	}
	return e.license.Payload.Tier
}

// License returns the underlying license.
func (e *Enforcer) License() *License {
	return e.license
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
