// Package features — trust_enforce implements Trust policy enforcement middleware.
// Policies are loaded from the trust_policies table and cached for 60 seconds.
// Policy types: "block" (reject response), "warn" (log + header), "log" (record only).
package features

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// TrustPolicy represents a loaded trust policy.
type TrustPolicy struct {
	ID      int64
	Name    string
	Type    string // "block", "warn", "log", "redact"
	Enabled bool
	Config  TrustPolicyConfig
}

// TrustPolicyConfig holds parsed policy configuration.
type TrustPolicyConfig struct {
	Description string `json:"description"`
	Pattern     string `json:"pattern"`     // regex pattern for content matching
	Target      string `json:"target"`      // "response" (default), "request", "both"
	MaxTokens   int    `json:"max_tokens"`  // max tokens per request (0 = unlimited)
	MaxCost     float64 `json:"max_cost"`   // max cost per request in USD (0 = unlimited)
	Models      string `json:"models"`      // comma-separated allowed/denied model list
	ModelsMode  string `json:"models_mode"` // "allow" or "deny"
}

// TrustEnforcer caches policies and enforces them on proxy requests.
type TrustEnforcer struct {
	db       *sql.DB
	mu       sync.RWMutex
	policies []TrustPolicy
	compiled map[string]*regexp.Regexp // name → compiled pattern
	loadedAt time.Time
	ttl      time.Duration
}

// NewTrustEnforcer creates a new enforcer backed by the given database.
func NewTrustEnforcer(db *sql.DB) *TrustEnforcer {
	te := &TrustEnforcer{
		db:       db,
		compiled: make(map[string]*regexp.Regexp),
		ttl:      60 * time.Second,
	}
	te.reload()
	return te
}

// reload fetches policies from the database.
func (te *TrustEnforcer) reload() {
	rows, err := te.db.Query(
		`SELECT id, name, type, config_json, enabled FROM trust_policies WHERE enabled = 1`,
	)
	if err != nil {
		log.Printf("[trust-enforce] reload error: %v", err)
		return
	}
	defer rows.Close()

	var policies []TrustPolicy
	compiled := make(map[string]*regexp.Regexp)

	for rows.Next() {
		var p TrustPolicy
		var configJSON string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &configJSON, &enabled); err != nil {
			log.Printf("[trust-enforce] scan error: %v", err)
			continue
		}
		p.Enabled = enabled == 1
		if err := json.Unmarshal([]byte(configJSON), &p.Config); err != nil {
			log.Printf("[trust-enforce] config parse error for %s: %v", p.Name, err)
		}

		// Compile regex pattern
		if p.Config.Pattern != "" {
			re, err := regexp.Compile("(?i)" + p.Config.Pattern)
			if err != nil {
				log.Printf("[trust-enforce] bad pattern for %s: %v", p.Name, err)
			} else {
				compiled[p.Name] = re
			}
		}

		policies = append(policies, p)
	}

	te.mu.Lock()
	te.policies = policies
	te.compiled = compiled
	te.loadedAt = time.Now()
	te.mu.Unlock()

	log.Printf("[trust-enforce] loaded %d policies", len(policies))
}

// getPolicies returns cached policies, reloading if stale.
func (te *TrustEnforcer) getPolicies() ([]TrustPolicy, map[string]*regexp.Regexp) {
	te.mu.RLock()
	if time.Since(te.loadedAt) < te.ttl {
		policies := te.policies
		compiled := te.compiled
		te.mu.RUnlock()
		return policies, compiled
	}
	te.mu.RUnlock()

	te.reload()

	te.mu.RLock()
	defer te.mu.RUnlock()
	return te.policies, te.compiled
}

// TrustPolicyViolation represents a policy violation.
type TrustPolicyViolation struct {
	Policy  string `json:"policy"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Middleware returns the proxy middleware that enforces trust policies.
func (te *TrustEnforcer) Middleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			policies, compiled := te.getPolicies()
			if len(policies) == 0 {
				return next(ctx, req)
			}

			// ── Pre-request checks ──────────────────────────────────
			for _, p := range policies {
				// Model allowlist/denylist
				if p.Config.Models != "" {
					models := strings.Split(p.Config.Models, ",")
					for i := range models {
						models[i] = strings.TrimSpace(models[i])
					}
					matched := modelInList(req.Model, models)
					if p.Config.ModelsMode == "deny" && matched {
						if p.Type == "block" {
							return nil, &TrustBlockError{
								Policy:  p.Name,
								Message: fmt.Sprintf("model %s is denied by policy %s", req.Model, p.Name),
							}
						}
					}
					if p.Config.ModelsMode == "allow" && !matched {
						if p.Type == "block" {
							return nil, &TrustBlockError{
								Policy:  p.Name,
								Message: fmt.Sprintf("model %s is not in allowed list for policy %s", req.Model, p.Name),
							}
						}
					}
				}

				// Request content pattern matching
				target := p.Config.Target
				if target == "" {
					target = "response"
				}
				if (target == "request" || target == "both") && compiled[p.Name] != nil {
					content := extractRequestContent(req)
					if compiled[p.Name].MatchString(content) {
						violation := TrustPolicyViolation{
							Policy:  p.Name,
							Type:    p.Type,
							Message: fmt.Sprintf("request matches policy %s pattern", p.Name),
						}
						if p.Type == "block" {
							recordViolation(te.db, p.Name, "block", req, nil)
							return nil, &TrustBlockError{Policy: p.Name, Message: violation.Message}
						}
						if p.Type == "warn" {
							recordViolation(te.db, p.Name, "warn", req, nil)
						}
					}
				}
			}

			// ── Execute request ──────────────────────────────────────
			resp, err := next(ctx, req)
			if err != nil || resp == nil {
				return resp, err
			}

			// ── Post-response checks ────────────────────────────────
			for _, p := range policies {
				// Response content pattern matching
				target := p.Config.Target
				if target == "" {
					target = "response"
				}
				if (target == "response" || target == "both") && compiled[p.Name] != nil {
					content := extractResponseContent(resp)
					if compiled[p.Name].MatchString(content) {
						violation := TrustPolicyViolation{
							Policy:  p.Name,
							Type:    p.Type,
							Message: fmt.Sprintf("response matches policy %s pattern", p.Name),
						}

						switch p.Type {
						case "block":
							recordViolation(te.db, p.Name, "block", req, resp)
							return nil, &TrustBlockError{Policy: p.Name, Message: violation.Message}

						case "redact":
							// Replace matched content in response choices
							for i := range resp.Choices {
								resp.Choices[i].Message.Content = compiled[p.Name].ReplaceAllString(
									resp.Choices[i].Message.Content, "[REDACTED]")
							}
							recordViolation(te.db, p.Name, "redact", req, resp)

						case "warn":
							log.Printf("[trust-enforce] WARNING: %s — %s", p.Name, violation.Message)
							recordViolation(te.db, p.Name, "warn", req, resp)

						case "log":
							recordViolation(te.db, p.Name, "log", req, resp)
						}
					}
				}

				// Token limit check
				if p.Config.MaxTokens > 0 && resp.Usage.TotalTokens > p.Config.MaxTokens {
					if p.Type == "block" {
						recordViolation(te.db, p.Name, "block", req, resp)
						return nil, &TrustBlockError{
							Policy:  p.Name,
							Message: fmt.Sprintf("token count %d exceeds limit %d", resp.Usage.TotalTokens, p.Config.MaxTokens),
						}
					}
					log.Printf("[trust-enforce] WARNING: %s — token count %d exceeds limit %d",
						p.Name, resp.Usage.TotalTokens, p.Config.MaxTokens)
					recordViolation(te.db, p.Name, p.Type, req, resp)
				}
			}

			return resp, nil
		}
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────

func extractRequestContent(req *provider.Request) string {
	var parts []string
	for _, m := range req.Messages {
		if m.Content != "" {
			parts = append(parts, m.Content)
		}
	}
	return strings.Join(parts, " ")
}

func extractResponseContent(resp *provider.Response) string {
	var parts []string
	for _, c := range resp.Choices {
		if c.Message.Content != "" {
			parts = append(parts, c.Message.Content)
		}
	}
	return strings.Join(parts, " ")
}

func modelInList(model string, list []string) bool {
	model = strings.ToLower(model)
	for _, m := range list {
		if strings.ToLower(m) == model {
			return true
		}
		// Support prefix matching: "gpt-4*" matches "gpt-4-turbo"
		if strings.HasSuffix(m, "*") && strings.HasPrefix(model, strings.ToLower(strings.TrimSuffix(m, "*"))) {
			return true
		}
	}
	return false
}

func recordViolation(db *sql.DB, policyName, action string, req *provider.Request, resp *provider.Response) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[trust-enforce] violation record panic: %v", r)
			}
		}()

		model := req.Model
		prov := req.Provider
		if resp != nil && resp.Model != "" {
			model = resp.Model
		}

		detail, _ := json.Marshal(map[string]any{
			"policy":   policyName,
			"action":   action,
			"model":    model,
			"provider": prov,
		})

		// Get previous hash for chain
		var prevHash string
		db.QueryRow("SELECT hash FROM trust_ledger ORDER BY id DESC LIMIT 1").Scan(&prevHash)

		now := time.Now().UTC().Format(time.RFC3339Nano)
		hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, "policy_violation", action, model, string(detail), now)
		h := sha256.Sum256([]byte(hashInput))
		hash := hex.EncodeToString(h[:])

		_, err := db.Exec(
			`INSERT INTO trust_ledger (event_type, actor, resource, action, detail_json, prev_hash, hash, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			"policy_violation", prov, model, action, string(detail), prevHash, hash, now,
		)
		if err != nil {
			log.Printf("[trust-enforce] ledger write error: %v", err)
		}
	}()
}

// TrustBlockError is returned when a trust policy blocks a request.
type TrustBlockError struct {
	Policy  string
	Message string
}

func (e *TrustBlockError) Error() string {
	return fmt.Sprintf("blocked by trust policy %s: %s", e.Policy, e.Message)
}
