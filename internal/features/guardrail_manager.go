package features

import (
	"database/sql"
	"log"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// GuardrailRule represents a configurable guardrail rule stored in the database.
type GuardrailRule struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`    // pii, injection, toxic, custom
	Pattern     string `json:"pattern"` // regex pattern
	Action      string `json:"action"`  // block, redact, warn, log
	Project     string `json:"project"` // "*" = all projects
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
}

// GuardrailMatch represents a match found by scanning text against rules.
type GuardrailMatch struct {
	RuleName string `json:"rule_name"`
	RuleType string `json:"rule_type"`
	Action   string `json:"action"`
	Match    string `json:"match"`
}

// GuardrailManager manages runtime-configurable guardrail rules loaded from the database.
type GuardrailManager struct {
	db       *sql.DB
	mu       sync.RWMutex
	rules    []GuardrailRule
	compiled map[string]*regexp.Regexp
	loadedAt time.Time
	ttl      time.Duration

	blocked  atomic.Int64
	flagged  atomic.Int64
	redacted atomic.Int64
	ruleHits sync.Map // rule name -> *atomic.Int64

	broadcaster EventBroadcaster
}

// NewGuardrailManager creates a new guardrail manager that loads rules from the database.
func NewGuardrailManager(db *sql.DB) *GuardrailManager {
	mgr := &GuardrailManager{
		db:       db,
		compiled: make(map[string]*regexp.Regexp),
		ttl:      60 * time.Second,
	}
	mgr.reload()
	return mgr
}

// SetBroadcaster sets the SSE event broadcaster.
func (mgr *GuardrailManager) SetBroadcaster(b EventBroadcaster) {
	mgr.broadcaster = b
}

// reload fetches rules from the database and compiles regex patterns.
func (mgr *GuardrailManager) reload() {
	rows, err := mgr.db.Query(`
		SELECT id, name, type, pattern, action, project, enabled, priority, description
		FROM guardrail_rules WHERE enabled = 1
		ORDER BY priority DESC, id ASC`)
	if err != nil {
		log.Printf("[guardrails] reload: %v", err)
		return
	}
	defer rows.Close()

	var rules []GuardrailRule
	compiled := make(map[string]*regexp.Regexp)

	for rows.Next() {
		var r GuardrailRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Pattern, &r.Action, &r.Project, &enabled, &r.Priority, &r.Description); err != nil {
			continue
		}
		r.Enabled = enabled == 1

		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			log.Printf("[guardrails] bad pattern for rule %q: %v", r.Name, err)
			continue
		}
		compiled[r.Name] = re
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	mgr.mu.Lock()
	mgr.rules = rules
	mgr.compiled = compiled
	mgr.loadedAt = time.Now()
	mgr.mu.Unlock()
}

// ensureFresh reloads rules if the TTL has expired.
func (mgr *GuardrailManager) ensureFresh() {
	mgr.mu.RLock()
	stale := time.Since(mgr.loadedAt) > mgr.ttl
	mgr.mu.RUnlock()
	if stale {
		mgr.reload()
	}
}

// GetRules returns all enabled rules applicable to the given project.
func (mgr *GuardrailManager) GetRules(project string) []GuardrailRule {
	mgr.ensureFresh()
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var result []GuardrailRule
	for _, r := range mgr.rules {
		if r.Project == "*" || r.Project == project {
			result = append(result, r)
		}
	}
	return result
}

// ScanText scans text against all applicable rules for a project and direction.
func (mgr *GuardrailManager) ScanText(text, project, direction string) []GuardrailMatch {
	mgr.ensureFresh()
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var matches []GuardrailMatch
	for _, r := range mgr.rules {
		if r.Project != "*" && r.Project != project {
			continue
		}
		re, ok := mgr.compiled[r.Name]
		if !ok {
			continue
		}
		found := re.FindString(text)
		if found == "" {
			continue
		}

		matches = append(matches, GuardrailMatch{
			RuleName: r.Name,
			RuleType: r.Type,
			Action:   r.Action,
			Match:    truncateMatch(found, 100),
		})

		// Update counters
		mgr.incrementRuleHits(r.Name)
		switch r.Action {
		case "block":
			mgr.blocked.Add(1)
		case "redact":
			mgr.redacted.Add(1)
		default:
			mgr.flagged.Add(1)
		}

		// Broadcast violation event
		if mgr.broadcaster != nil {
			mgr.broadcaster.Send(map[string]any{
				"type":      "guardrail_violation",
				"rule":      r.Name,
				"rule_type": r.Type,
				"action":    r.Action,
				"direction": direction,
				"project":   project,
			})
		}

		// Fire webhook
		fireWebhook("guardrail.violation", map[string]any{
			"rule":      r.Name,
			"rule_type": r.Type,
			"action":    r.Action,
			"direction": direction,
			"project":   project,
		})
	}
	return matches
}

func (mgr *GuardrailManager) incrementRuleHits(name string) {
	v, _ := mgr.ruleHits.LoadOrStore(name, &atomic.Int64{})
	v.(*atomic.Int64).Add(1)
}

// RecordEvent persists a guardrail violation event to the database.
func (mgr *GuardrailManager) RecordEvent(ruleName, ruleType, action, direction, project, model, matchText, requestID string) {
	mgr.db.Exec(`
		INSERT INTO guardrail_events (rule_name, rule_type, action, direction, project, model, match_text, request_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ruleName, ruleType, action, direction, project, model, truncateMatch(matchText, 200), requestID)
}

// Stats returns current analytics counters.
func (mgr *GuardrailManager) Stats() map[string]any {
	ruleHits := make(map[string]int64)
	mgr.ruleHits.Range(func(key, value any) bool {
		ruleHits[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})
	return map[string]any{
		"blocked":   mgr.blocked.Load(),
		"flagged":   mgr.flagged.Load(),
		"redacted":  mgr.redacted.Load(),
		"rule_hits": ruleHits,
	}
}

// Invalidate forces a reload of rules from the database.
func (mgr *GuardrailManager) Invalidate() {
	mgr.reload()
}
