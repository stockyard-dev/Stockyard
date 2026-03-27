// Package engine — upgrade_events fires webhook events at natural conversion
// boundaries. When a free user hits a paywall moment, the webhook fires so
// external systems (Slack, email, CRM) can act on it.
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// GlobalWebhookMgr is set during Boot() for package-level event firing.
var GlobalWebhookMgr *WebhookManager

// GlobalSSEBroadcaster is set during Boot() for in-app notifications.
var GlobalSSEBroadcaster webhookBroadcaster

// upgradeEventTracker prevents duplicate events within a cooldown window.
type upgradeEventTracker struct {
	mu       sync.Mutex
	lastFired map[string]time.Time
}

var eventTracker = &upgradeEventTracker{
	lastFired: make(map[string]time.Time),
}

// cooldown prevents the same event type from firing more than once per window.
func (t *upgradeEventTracker) shouldFire(eventType string, cooldown time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	last, ok := t.lastFired[eventType]
	if ok && time.Since(last) < cooldown {
		return false
	}
	t.lastFired[eventType] = time.Now()
	return true
}

// FireUpgradeEvent dispatches a conversion event via webhooks and SSE.
// It deduplicates events within a cooldown window to prevent spam.
func FireUpgradeEvent(eventType, title, message string, data map[string]any, cooldown time.Duration) {
	if GlobalWebhookMgr == nil {
		return
	}

	if !eventTracker.shouldFire(eventType, cooldown) {
		return
	}

	log.Printf("[upgrade-event] %s: %s", eventType, title)

	event := WebhookEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data: map[string]any{
			"title":   title,
			"message": message,
			"details": data,
		},
	}

	// Fire webhooks (async, with retries)
	go GlobalWebhookMgr.Fire(context.Background(), event)

	// Broadcast via SSE for in-app notifications
	if GlobalSSEBroadcaster != nil {
		GlobalSSEBroadcaster.Send(map[string]any{
			"type":      "upgrade_prompt",
			"event":     eventType,
			"title":     title,
			"message":   message,
			"details":   data,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ── Specific event triggers ────────────────────────────────────────────────

// FireDroverCapReached fires when a free-tier user hits the 100/day Drover cap.
func FireDroverCapReached(usedToday int64, savingsToday float64) {
	FireUpgradeEvent(
		"upgrade.drover_cap_reached",
		"Drover daily cap reached",
		fmt.Sprintf("Drover saved you $%.2f today across %d requests. Upgrade to Pro for unlimited autopilot routing.", savingsToday, usedToday),
		map[string]any{
			"used_today":    usedToday,
			"savings_today": savingsToday,
			"product":       "Drover",
			"target_tier":   "Pro",
			"target_price":  "$99.99/mo",
			"upgrade_url":   "/pricing/",
		},
		1*time.Hour, // max once per hour
	)
}

// FireDroverSavingsMilestone fires at savings milestones ($1, $5, $10, $50).
func FireDroverSavingsMilestone(totalSavings float64, milestone float64) {
	FireUpgradeEvent(
		fmt.Sprintf("upgrade.drover_savings_%.0f", milestone),
		fmt.Sprintf("Drover has saved you $%.2f", totalSavings),
		fmt.Sprintf("You've saved $%.2f total with Drover autopilot routing. Upgrade to Pro for unlimited routing and save even more.", totalSavings),
		map[string]any{
			"total_savings": totalSavings,
			"milestone":     milestone,
			"product":       "Drover",
			"target_tier":   "Pro",
			"target_price":  "$99.99/mo",
			"upgrade_url":   "/drover/",
		},
		24*time.Hour, // max once per day per milestone
	)
}

// FireFeralBypassDetected fires when a quickscan finds vulnerabilities.
func FireFeralBypassDetected(bypassed, total int, grade string) {
	FireUpgradeEvent(
		"upgrade.feral_bypass_detected",
		fmt.Sprintf("Security audit: %d/%d probes bypassed (Grade %s)", bypassed, total, grade),
		fmt.Sprintf("%d of %d probes bypassed your defenses. Your stack is vulnerable to prompt injection. Unlock the full 29-probe Feral suite to find and fix all vulnerabilities.", bypassed, total),
		map[string]any{
			"bypassed":     bypassed,
			"total":        total,
			"grade":        grade,
			"product":      "Feral",
			"target_tier":  "Team",
			"target_price": "$299.99/mo",
			"upgrade_url":  "/feral/",
		},
		1*time.Hour,
	)
}

// FirePIIDetected fires when auto-insights detects PII in traces.
func FirePIIDetected(affectedTraces, totalScanned int, piiTypes map[string]int) {
	FireUpgradeEvent(
		"upgrade.pii_detected",
		fmt.Sprintf("PII detected in %d of %d recent requests", affectedTraces, totalScanned),
		fmt.Sprintf("Personally identifiable information was found in %d of %d recent requests. This data is being sent to third-party LLM providers. Unlock the safety suite with Team to auto-detect and redact PII.", affectedTraces, totalScanned),
		map[string]any{
			"affected_traces": affectedTraces,
			"total_scanned":   totalScanned,
			"pii_types":       piiTypes,
			"product":         "Brand",
			"target_tier":     "Team",
			"target_price":    "$299.99/mo",
			"upgrade_url":     "/pricing/",
		},
		6*time.Hour,
	)
}

// FireTraceMilestone fires at trace count milestones (100, 500, 1000, 5000).
func FireTraceMilestone(traceCount int64, milestone int64) {
	FireUpgradeEvent(
		fmt.Sprintf("upgrade.trace_milestone_%d", milestone),
		fmt.Sprintf("You've sent %d requests through Stockyard", traceCount),
		fmt.Sprintf("Milestone: %d requests. You're generating valuable data. Run a Feral quickscan to check your security, or enable Drover to start saving on every request.", traceCount),
		map[string]any{
			"trace_count": traceCount,
			"milestone":   milestone,
			"actions": []map[string]string{
				{"action": "Run quickscan", "url": "/feral/", "tier": "Free"},
				{"action": "Try Drover", "url": "/drover/", "tier": "Free (100/day)"},
				{"action": "See insights", "url": "/api/insights", "tier": "Free"},
			},
		},
		24*time.Hour,
	)
}

// FireErrorRateHigh fires when error rate crosses 10%.
func FireErrorRateHigh(errorRate float64, errorCount, totalRequests int, worstProvider string) {
	FireUpgradeEvent(
		"upgrade.error_rate_high",
		fmt.Sprintf("%.1f%% error rate detected", errorRate),
		fmt.Sprintf("%d of %d requests failed (%.1f%% error rate). Provider '%s' has the most errors. Enable failover routing to automatically retry on backup providers.", errorCount, totalRequests, errorRate, worstProvider),
		map[string]any{
			"error_rate":      errorRate,
			"error_count":     errorCount,
			"total_requests":  totalRequests,
			"worst_provider":  worstProvider,
			"product":         "Chute",
			"action":          "Enable failover module",
			"target_tier":     "Community",
			"target_price":    "Free",
		},
		6*time.Hour,
	)
}
