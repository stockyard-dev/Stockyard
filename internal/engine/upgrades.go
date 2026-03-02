package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

// GlobalUsageCounters is the package-level usage tracker, initialized in Boot().
var GlobalUsageCounters *UsageCounters

// UpgradeTrigger is a single upgrade prompt to display in the dashboard.
type UpgradeTrigger struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"` // 1=highest
	Title    string `json:"title"`
	Message  string `json:"message"`
	CTA      string `json:"cta"`
	CTALink  string `json:"cta_link"`
	Tier     string `json:"target_tier"` // which tier to upgrade to
}

// UsageCounters tracks per-month usage for upgrade trigger evaluation.
type UsageCounters struct {
	mu sync.RWMutex
	db *sql.DB

	// In-memory cache (flushed to SQLite periodically)
	month           string // "2026-03"
	requestCount    int64
	cacheHits       int64
	cacheSavingsUSD float64
	providersUsed   map[string]bool
	modulesBlocked  int64 // attempts to use modules beyond tier limit
	apiKeysCount    int
	bootCount       int64
}

// NewUsageCounters initializes the usage tracking system.
func NewUsageCounters(db *sql.DB) *UsageCounters {
	uc := &UsageCounters{
		db:            db,
		month:         time.Now().Format("2006-01"),
		providersUsed: make(map[string]bool),
	}
	uc.migrate()
	uc.load()
	return uc
}

func (uc *UsageCounters) migrate() {
	_, err := uc.db.Exec(`CREATE TABLE IF NOT EXISTS usage_counters (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		log.Printf("[upgrades] migration error: %v", err)
	}
}

func (uc *UsageCounters) load() {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	month := time.Now().Format("2006-01")
	uc.month = month

	uc.requestCount = uc.getInt("requests:" + month)
	uc.cacheHits = uc.getInt("cache_hits:" + month)
	uc.cacheSavingsUSD = uc.getFloat("cache_savings:" + month)
	uc.bootCount = uc.getInt("boot_count")

	// Load providers
	if prov := uc.getStr("providers:" + month); prov != "" {
		for _, p := range strings.Split(prov, ",") {
			if p != "" {
				uc.providersUsed[p] = true
			}
		}
	}
}

func (uc *UsageCounters) getInt(key string) int64 {
	var val string
	err := uc.db.QueryRow("SELECT value FROM usage_counters WHERE key=?", key).Scan(&val)
	if err != nil {
		return 0
	}
	var n int64
	fmt.Sscanf(val, "%d", &n)
	return n
}

func (uc *UsageCounters) getFloat(key string) float64 {
	var val string
	err := uc.db.QueryRow("SELECT value FROM usage_counters WHERE key=?", key).Scan(&val)
	if err != nil {
		return 0
	}
	var f float64
	fmt.Sscanf(val, "%f", &f)
	return f
}

func (uc *UsageCounters) getStr(key string) string {
	var val string
	err := uc.db.QueryRow("SELECT value FROM usage_counters WHERE key=?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func (uc *UsageCounters) set(key, value string) {
	uc.db.Exec(`INSERT INTO usage_counters (key, value, updated_at) VALUES (?,?,datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, value)
}

// RecordRequest increments the monthly request counter.
func (uc *UsageCounters) RecordRequest(provider string, cacheHit bool, costSaved float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	month := time.Now().Format("2006-01")
	if month != uc.month {
		// Month rolled over — reset counters
		uc.month = month
		uc.requestCount = 0
		uc.cacheHits = 0
		uc.cacheSavingsUSD = 0
		uc.providersUsed = make(map[string]bool)
	}

	uc.requestCount++
	uc.set("requests:"+month, fmt.Sprintf("%d", uc.requestCount))

	if provider != "" {
		uc.providersUsed[provider] = true
		provList := make([]string, 0, len(uc.providersUsed))
		for p := range uc.providersUsed {
			provList = append(provList, p)
		}
		uc.set("providers:"+month, strings.Join(provList, ","))
	}

	if cacheHit {
		uc.cacheHits++
		uc.cacheSavingsUSD += costSaved
		uc.set("cache_hits:"+month, fmt.Sprintf("%d", uc.cacheHits))
		uc.set("cache_savings:"+month, fmt.Sprintf("%.4f", uc.cacheSavingsUSD))
	}
}

// RecordBoot increments the boot/restart counter.
func (uc *UsageCounters) RecordBoot() {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.bootCount++
	uc.set("boot_count", fmt.Sprintf("%d", uc.bootCount))
}

// RecordModuleBlock tracks attempts to use gated modules.
func (uc *UsageCounters) RecordModuleBlock() {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.modulesBlocked++
	month := time.Now().Format("2006-01")
	uc.set("module_blocks:"+month, fmt.Sprintf("%d", uc.modulesBlocked))
}

// Snapshot returns current counters for evaluation.
func (uc *UsageCounters) Snapshot() map[string]any {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return map[string]any{
		"month":            uc.month,
		"requests":         uc.requestCount,
		"cache_hits":       uc.cacheHits,
		"cache_savings":    uc.cacheSavingsUSD,
		"providers_used":   len(uc.providersUsed),
		"providers_list":   uc.providersList(),
		"modules_blocked":  uc.modulesBlocked,
		"boot_count":       uc.bootCount,
	}
}

func (uc *UsageCounters) providersList() []string {
	list := make([]string, 0, len(uc.providersUsed))
	for p := range uc.providersUsed {
		list = append(list, p)
	}
	return list
}

// EvaluateTriggers runs the rules engine and returns active upgrade prompts.
func EvaluateTriggers(enforcer *license.Enforcer, counters *UsageCounters) []UpgradeTrigger {
	tier := enforcer.Tier()
	limits := license.Limits(tier)
	snap := counters.Snapshot()
	var triggers []UpgradeTrigger

	requests := snap["requests"].(int64)
	cacheHits := snap["cache_hits"].(int64)
	cacheSavings := snap["cache_savings"].(float64)
	providersUsed := snap["providers_used"].(int)
	modulesBlocked := snap["modules_blocked"].(int64)
	bootCount := snap["boot_count"].(int64)

	// Paid tiers don't see upgrade prompts (except Team → Enterprise)
	switch tier {
	case license.TierPro, license.TierCloud:
		// Only show team detection
		if apiKeyCount := countAPIKeys(counters.db); apiKeyCount >= 2 {
			triggers = append(triggers, UpgradeTrigger{
				ID:       "team_detection",
				Priority: 3,
				Title:    "Team detected",
				Message:  fmt.Sprintf("You have %d API keys active. The Team plan gives you a shared dashboard, team-level audit trails, and 5 seats for $149/mo.", apiKeyCount),
				CTA:      "Explore Team",
				CTALink:  "/pricing/",
				Tier:     "team",
			})
		}
		return triggers
	case license.TierTeam, license.TierEnterprise:
		return triggers // no prompts
	}

	// === Community tier triggers ===

	// 1. Usage ceiling (high priority — creates urgency)
	if limits.MaxRequestsPerMonth > 0 {
		pct := float64(requests) / float64(limits.MaxRequestsPerMonth) * 100
		if pct >= 80 {
			remaining := limits.MaxRequestsPerMonth - requests
			triggers = append(triggers, UpgradeTrigger{
				ID:       "usage_ceiling",
				Priority: 1,
				Title:    "Approaching request limit",
				Message:  fmt.Sprintf("You've used %d of %d requests this month (%d remaining). Upgrade to Individual for 10,000 requests/mo.", requests, limits.MaxRequestsPerMonth, remaining),
				CTA:      "Upgrade to Individual — $9.99/mo",
				CTALink:  "/pricing/",
				Tier:     "individual",
			})
		}
	}

	// 2. Cost savings proof (very high — shows $ value)
	if cacheHits >= 50 && cacheSavings > 0 {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "cost_savings",
			Priority: 2,
			Title:    "Cache is saving you money",
			Message:  fmt.Sprintf("Cache has saved you $%.2f this month across %d hits. Unlock all 70 modules with Individual to save even more.", cacheSavings, cacheHits),
			CTA:      "Unlock all modules — $9.99/mo",
			CTALink:  "/pricing/",
			Tier:     "individual",
		})
	}

	// 3. Provider lock (hits at point of need)
	if limits.MaxProviders > 0 && providersUsed >= limits.MaxProviders {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "provider_lock",
			Priority: 2,
			Title:    "Provider limit reached",
			Message:  fmt.Sprintf("Community tier includes %d providers. You've used %d. Unlock all 16 providers with Individual.", limits.MaxProviders, providersUsed),
			CTA:      "Unlock all providers — $9.99/mo",
			CTALink:  "/pricing/",
			Tier:     "individual",
		})
	}

	// 4. Module gate (feature discovery)
	if modulesBlocked > 0 {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "module_gate",
			Priority: 3,
			Title:    "Premium modules available",
			Message:  fmt.Sprintf("You've tried to enable %d premium modules. Community includes 20 basic modules — Individual unlocks all 70.", modulesBlocked),
			CTA:      "Unlock all modules — $9.99/mo",
			CTALink:  "/pricing/",
			Tier:     "individual",
		})
	}

	// 5. Audit retention expiry (loss aversion)
	if limits.RetentionDays > 0 && requests > 100 {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "audit_expiry",
			Priority: 4,
			Title:    "Audit trail expiring",
			Message:  fmt.Sprintf("Community tier keeps %d days of audit history. Your earliest records are being archived. Pro retains 90 days.", limits.RetentionDays),
			CTA:      "Keep 90 days with Pro — $49/mo",
			CTALink:  "/pricing/",
			Tier:     "pro",
		})
	}

	// 6. Team detection (captures expansion)
	if apiKeyCount := countAPIKeys(counters.db); apiKeyCount >= 2 {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "team_detection",
			Priority: 3,
			Title:    "Team detected",
			Message:  fmt.Sprintf("You have %d API keys active. The Team plan includes a shared dashboard, team-level audit trails, and 5 seats.", apiKeyCount),
			CTA:      "Explore Team — $149/mo",
			CTALink:  "/pricing/",
			Tier:     "team",
		})
	}

	// 7. Cloud convenience (pain-point driven)
	if bootCount >= 3 && (tier == license.TierCommunity || tier == license.TierIndividual) {
		triggers = append(triggers, UpgradeTrigger{
			ID:       "cloud_convenience",
			Priority: 5,
			Title:    "Tired of managing infra?",
			Message:  fmt.Sprintf("This instance has restarted %d times. Pro Cloud gives you zero-ops hosting — we manage everything. 30-second migration.", bootCount),
			CTA:      "Go Cloud — $49/mo",
			CTALink:  "/cloud/",
			Tier:     "pro",
		})
	}

	// Individual tier → Pro upsell
	if tier == license.TierIndividual {
		// Clear community-tier triggers and add Individual→Pro
		triggers = nil

		if bootCount >= 3 {
			triggers = append(triggers, UpgradeTrigger{
				ID:       "cloud_convenience",
				Priority: 2,
				Title:    "Tired of managing infra?",
				Message:  fmt.Sprintf("This instance has restarted %d times. Pro Cloud gives you zero-ops hosting with auto backups and 90-day retention.", bootCount),
				CTA:      "Go Pro — $49/mo",
				CTALink:  "/cloud/",
				Tier:     "pro",
			})
		}

		if requests > 5000 {
			triggers = append(triggers, UpgradeTrigger{
				ID:       "heavy_usage",
				Priority: 3,
				Title:    "Heavy usage detected",
				Message:  fmt.Sprintf("You've made %d requests this month. Pro gives you unlimited requests, cloud hosting, and 90-day audit retention.", requests),
				CTA:      "Upgrade to Pro — $49/mo",
				CTALink:  "/pricing/",
				Tier:     "pro",
			})
		}
	}

	// Sort by priority (lower = higher priority)
	for i := 0; i < len(triggers); i++ {
		for j := i + 1; j < len(triggers); j++ {
			if triggers[j].Priority < triggers[i].Priority {
				triggers[i], triggers[j] = triggers[j], triggers[i]
			}
		}
	}

	// Return max 3 triggers
	if len(triggers) > 3 {
		triggers = triggers[:3]
	}

	return triggers
}

// countAPIKeys checks how many API keys exist in the auth system.
func countAPIKeys(db *sql.DB) int {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE revoked=0").Scan(&count)
	if err != nil {
		// Table might not exist
		return 0
	}
	return count
}

// UpgradePromptsHandler returns an HTTP handler for GET /api/upgrade-prompts.
func UpgradePromptsHandler(enforcer *license.Enforcer, counters *UsageCounters) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		triggers := EvaluateTriggers(enforcer, counters)
		usage := counters.Snapshot()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tier":     string(enforcer.Tier()),
			"triggers": triggers,
			"count":    len(triggers),
			"usage":    usage,
		})
	}
}
