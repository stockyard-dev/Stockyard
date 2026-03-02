package license

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TierLimits defines the operational limits for a pricing tier.
type TierLimits struct {
	MaxRequestsPerMonth int64 // 0 = unlimited
	MaxProviders        int   // 0 = unlimited
	MaxModules          int   // 0 = unlimited
	MaxUsers            int   // 0 = unlimited
	RetentionDays       int   // 0 = unlimited
	EmailAlerts         bool
	AutoBackups         bool
	PrioritySupport     bool
}

// Limits returns the TierLimits for a given tier.
func Limits(tier Tier) TierLimits {
	switch tier {
	case TierIndividual:
		return TierLimits{
			MaxRequestsPerMonth: 10_000,
			MaxProviders:        16, // all
			MaxModules:          70, // all
			MaxUsers:            1,
			RetentionDays:       30,
			EmailAlerts:         true,
			AutoBackups:         false,
			PrioritySupport:     false,
		}
	case TierPro:
		return TierLimits{
			MaxRequestsPerMonth: 0, // unlimited
			MaxProviders:        16,
			MaxModules:          70,
			MaxUsers:            1,
			RetentionDays:       90,
			EmailAlerts:         true,
			AutoBackups:         true,
			PrioritySupport:     true,
		}
	case TierTeam:
		return TierLimits{
			MaxRequestsPerMonth: 0, // unlimited
			MaxProviders:        16,
			MaxModules:          70,
			MaxUsers:            5, // included, +$25/seat after
			RetentionDays:       365,
			EmailAlerts:         true,
			AutoBackups:         true,
			PrioritySupport:     true,
		}
	case TierCloud:
		return TierLimits{
			MaxRequestsPerMonth: 0, // unlimited (legacy, maps to Pro)
			MaxProviders:        16,
			MaxModules:          70,
			MaxUsers:            0,
			RetentionDays:       90,
			EmailAlerts:         true,
			AutoBackups:         true,
			PrioritySupport:     true,
		}
	case TierEnterprise:
		return TierLimits{
			MaxRequestsPerMonth: 0,
			MaxProviders:        16,
			MaxModules:          70,
			MaxUsers:            0, // unlimited
			RetentionDays:       0, // unlimited
			EmailAlerts:         true,
			AutoBackups:         true,
			PrioritySupport:     true,
		}
	default: // TierCommunity (no license key)
		return TierLimits{
			MaxRequestsPerMonth: 1_000,
			MaxProviders:        3,
			MaxModules:          20,
			MaxUsers:            1,
			RetentionDays:       7,
			EmailAlerts:         false,
			AutoBackups:         false,
			PrioritySupport:     false,
		}
	}
}

// Enforcer tracks usage and enforces tier limits at runtime.
type Enforcer struct {
	license *License
	limits  TierLimits

	mu      sync.Mutex
	moStart time.Time

	monthCount atomic.Int64
	totalCount atomic.Int64
	blocked    atomic.Int64
}

// NewEnforcer creates an Enforcer for the given license.
func NewEnforcer(lic *License) *Enforcer {
	now := time.Now()
	return &Enforcer{
		license: lic,
		limits:  Limits(lic.Payload.Tier),
		moStart: startOfMonth(now),
	}
}

// Check determines if a request is allowed under the current tier limits.
// Returns nil if allowed, or an error describing why the request was blocked.
func (e *Enforcer) Check() error {
	// Expired license = community tier limits
	if e.license.IsExpired() {
		return fmt.Errorf("license expired on %s — operating in community tier", e.license.ExpiresAt.Format("2006-01-02"))
	}

	now := time.Now()

	// Reset counter on month rollover
	e.mu.Lock()
	if now.After(e.moStart.AddDate(0, 1, 0)) {
		e.monthCount.Store(0)
		e.moStart = startOfMonth(now)
	}
	e.mu.Unlock()

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
	e.monthCount.Add(1)
	e.totalCount.Add(1)

	return nil
}

// CheckUserLimit returns an error if adding a user would exceed the tier's user cap.
func (e *Enforcer) CheckUserLimit(currentUsers int) error {
	if e.limits.MaxUsers > 0 && currentUsers >= e.limits.MaxUsers {
		return fmt.Errorf("user limit reached (%d/%d) — upgrade at stockyard.dev/pricing",
			currentUsers, e.limits.MaxUsers)
	}
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
		"requests_month":   e.monthCount.Load(),
		"requests_total":   e.totalCount.Load(),
		"requests_blocked": e.blocked.Load(),
		"monthly_limit":    lim.MaxRequestsPerMonth,
		"max_users":        lim.MaxUsers,
		"retention_days":   lim.RetentionDays,
		"email_alerts":     lim.EmailAlerts,
		"auto_backups":     lim.AutoBackups,
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
		return TierCommunity
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
