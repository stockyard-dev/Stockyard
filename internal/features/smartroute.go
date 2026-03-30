package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// SmartRouteSchema creates the routing_rules table.
const SmartRouteSchema = `
CREATE TABLE IF NOT EXISTS routing_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    condition TEXT NOT NULL DEFAULT '{}',
    action TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_routing_rules_priority ON routing_rules(priority);
`

// RoutingRule represents a configurable routing rule.
type RoutingRule struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Priority  int             `json:"priority"`
	Condition json.RawMessage `json:"condition"`
	Action    json.RawMessage `json:"action"`
	Enabled   bool            `json:"enabled"`
	CreatedAt string          `json:"created_at"`
}

// routeCondition is the parsed condition JSON.
type routeCondition struct {
	Field string `json:"field"` // prompt_length, model, provider, time_of_day, ab_split
	Op    string `json:"op"`    // <, >, <=, >=, ==, !=, contains
	Value any    `json:"value"` // numeric or string depending on field
}

// routeAction is the parsed action JSON.
type routeAction struct {
	RouteToModel    string `json:"route_to_model"`
	RouteToProvider string `json:"route_to_provider,omitempty"`
}

// SmartRouter manages routing rules with in-memory caching.
type SmartRouter struct {
	conn     *sql.DB
	mu       sync.RWMutex
	rules    []RoutingRule
	lastLoad time.Time
	cacheTTL time.Duration
}

// NewSmartRouter creates a new smart router with DB-backed rules.
func NewSmartRouter(conn *sql.DB) *SmartRouter {
	// Ensure schema exists.
	if _, err := conn.Exec(SmartRouteSchema); err != nil {
		log.Printf("[schema] migration error: %v", err)
	}

	sr := &SmartRouter{
		conn:     conn,
		cacheTTL: 60 * time.Second,
	}
	sr.reload()
	return sr
}

func (sr *SmartRouter) reload() {
	rows, err := sr.conn.Query("SELECT id, name, priority, condition, action, enabled, created_at FROM routing_rules WHERE enabled = 1 ORDER BY priority DESC")
	if err != nil {
		log.Printf("[smartroute] reload error: %v", err)
		return
	}
	defer rows.Close()

	var rules []RoutingRule
	for rows.Next() {
		var r RoutingRule
		var enabled int
		var condStr, actStr string
		if err := rows.Scan(&r.ID, &r.Name, &r.Priority, &condStr, &actStr, &enabled, &r.CreatedAt); err != nil {
			continue
		}
		r.Condition = json.RawMessage(condStr)
		r.Action = json.RawMessage(actStr)
		r.Enabled = enabled == 1
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	sr.mu.Lock()
	sr.rules = rules
	sr.lastLoad = time.Now()
	sr.mu.Unlock()
}

func (sr *SmartRouter) getRules() []RoutingRule {
	sr.mu.RLock()
	if time.Since(sr.lastLoad) > sr.cacheTTL {
		sr.mu.RUnlock()
		sr.reload()
		sr.mu.RLock()
	}
	rules := make([]RoutingRule, len(sr.rules))
	copy(rules, sr.rules)
	sr.mu.RUnlock()
	return rules
}

// Evaluate checks if a request matches a rule's condition.
func (sr *SmartRouter) evaluate(cond routeCondition, req *provider.Request) bool {
	switch cond.Field {
	case "prompt_length":
		length := 0
		for _, m := range req.Messages {
			length += len(m.Content)
		}
		return compareNumeric(float64(length), cond.Op, toFloat(cond.Value))

	case "model":
		return compareString(req.Model, cond.Op, toString(cond.Value))

	case "provider":
		return compareString(req.Provider, cond.Op, toString(cond.Value))

	case "time_of_day":
		hour := time.Now().UTC().Hour()
		return compareNumeric(float64(hour), cond.Op, toFloat(cond.Value))

	case "ab_split":
		// Percentage-based split: value is 0-100.
		pct := int(toFloat(cond.Value))
		if pct <= 0 {
			return false
		}
		if pct >= 100 {
			return true
		}
		n, _ := rand.Int(rand.Reader, big.NewInt(100))
		return int(n.Int64()) < pct

	default:
		return false
	}
}

// SmartRouteMiddleware creates a proxy middleware that applies routing rules.
func SmartRouteMiddleware(sr *SmartRouter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			rules := sr.getRules()

			for _, rule := range rules {
				var cond routeCondition
				if err := json.Unmarshal(rule.Condition, &cond); err != nil {
					continue
				}

				if !sr.evaluate(cond, req) {
					continue
				}

				var act routeAction
				if err := json.Unmarshal(rule.Action, &act); err != nil {
					continue
				}

				if act.RouteToModel != "" {
					if req.Extra == nil {
						req.Extra = make(map[string]any)
					}
					req.Extra["_smart_route_rule"] = rule.Name
					req.Extra["_smart_route_original_model"] = req.Model

					// Tag A/B variant.
					if cond.Field == "ab_split" {
						req.Extra["_ab_variant"] = "treatment"
						req.Extra["_ab_rule_id"] = rule.ID
					}

					req.Model = act.RouteToModel
					if act.RouteToProvider != "" {
						req.Provider = act.RouteToProvider
					}

					log.Printf("[smartroute] rule %q: %s → %s", rule.Name, req.Extra["_smart_route_original_model"], act.RouteToModel)
					break // First matching rule wins.
				}
			}

			// If no A/B rule matched, tag as control for any ab_split rules that exist.
			if req.Extra != nil && req.Extra["_ab_variant"] == nil {
				for _, rule := range rules {
					var cond routeCondition
					if err := json.Unmarshal(rule.Condition, &cond); err != nil {
						log.Printf("[smartroute] json parse error: %v", err)
					}
					if cond.Field == "ab_split" {
						if req.Extra == nil {
							req.Extra = make(map[string]any)
						}
						req.Extra["_ab_variant"] = "control"
						req.Extra["_ab_rule_id"] = rule.ID
						break
					}
				}
			}

			return next(ctx, req)
		}
	}
}

// --- Helpers ---

func compareNumeric(actual float64, op string, expected float64) bool {
	switch op {
	case "<":
		return actual < expected
	case ">":
		return actual > expected
	case "<=":
		return actual <= expected
	case ">=":
		return actual >= expected
	case "==", "eq":
		return actual == expected
	case "!=":
		return actual != expected
	default:
		return false
	}
}

func compareString(actual, op, expected string) bool {
	switch op {
	case "==", "eq":
		return actual == expected
	case "!=":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "prefix":
		return strings.HasPrefix(actual, expected)
	default:
		return false
	}
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0
	}
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func generateRuleID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "rr_" + hex.EncodeToString(b)
}
