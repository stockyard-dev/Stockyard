package features

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// compiledRule is a pre-processed routing rule.
type compiledRule struct {
	config.ModelRouteRule
	regex *regexp.Regexp // compiled if condition is "pattern"
}

// RouteStats tracks how many requests each route handles.
type RouteStats struct {
	Requests atomic.Int64
	Tokens   atomic.Int64
}

// ModelRouter manages intelligent model routing.
type ModelRouter struct {
	mu       sync.RWMutex
	rules    []compiledRule
	stats    map[string]*RouteStats // route name → stats
	fallback string
}

// NewModelRouter creates a model router from config.
func NewModelRouter(cfg config.ModelSwitchConfig) *ModelRouter {
	mr := &ModelRouter{
		stats:    make(map[string]*RouteStats),
		fallback: cfg.Default,
	}

	for _, rule := range cfg.Rules {
		cr := compiledRule{ModelRouteRule: rule}
		if rule.Condition == "pattern" && rule.Value != "" {
			compiled, err := regexp.Compile(rule.Value)
			if err != nil {
				log.Printf("modelswitch: invalid pattern %q in rule %q: %v", rule.Value, rule.Name, err)
				continue
			}
			cr.regex = compiled
		}
		mr.rules = append(mr.rules, cr)
		mr.stats[rule.Name] = &RouteStats{}
	}

	return mr
}

// Route evaluates all rules against a request and returns the target model and provider.
// Returns empty strings if no rule matches (use fallback/original).
func (mr *ModelRouter) Route(req *provider.Request) (model string, prov string, ruleName string) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	// Compute input token count for token-based rules
	inputTokens := tracker.CountInputTokens(req.Model, req.Messages)

	// Concatenate all message content for pattern matching
	var allContent strings.Builder
	for _, msg := range req.Messages {
		allContent.WriteString(msg.Content)
		allContent.WriteByte(' ')
	}
	fullText := allContent.String()

	// Evaluate rules in priority order
	for _, rule := range mr.rules {
		matched := false

		switch rule.Condition {
		case "token_count":
			threshold, err := strconv.Atoi(rule.Value)
			if err != nil {
				continue
			}
			matched = evaluateOperator(inputTokens, threshold, rule.Operator)

		case "pattern":
			if rule.regex != nil {
				found := rule.regex.MatchString(fullText)
				matched = (rule.Operator == "matches" && found) || (rule.Operator == "not_matches" && !found)
			}

		case "contains":
			found := strings.Contains(strings.ToLower(fullText), strings.ToLower(rule.Value))
			matched = (rule.Operator == "contains" || rule.Operator == "eq") && found

		case "header":
			// Check X-Route-Hint header
			hint, _ := req.Extra["_route_hint"].(string)
			matched = strings.EqualFold(hint, rule.Value)

		case "model":
			// Match against the requested model
			matched = strings.EqualFold(req.Model, rule.Value)

		case "always":
			matched = true
		}

		if !matched {
			continue
		}

		// A/B testing: weight-based probability
		if rule.Weight > 0 && rule.Weight < 100 {
			if rand.Intn(100) >= rule.Weight {
				continue // Didn't win the dice roll
			}
		}

		// Rule matched
		if stat, ok := mr.stats[rule.Name]; ok {
			stat.Requests.Add(1)
			stat.Tokens.Add(int64(inputTokens))
		}

		return rule.Model, rule.Provider, rule.Name
	}

	return "", "", ""
}

// Stats returns routing statistics per rule.
func (mr *ModelRouter) Stats() []map[string]any {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var result []map[string]any
	for _, rule := range mr.rules {
		stat := mr.stats[rule.Name]
		result = append(result, map[string]any{
			"name":      rule.Name,
			"condition": rule.Condition,
			"model":     rule.Model,
			"provider":  rule.Provider,
			"weight":    rule.Weight,
			"requests":  stat.Requests.Load(),
			"tokens":    stat.Tokens.Load(),
		})
	}
	return result
}

func evaluateOperator(actual, threshold int, op string) bool {
	switch op {
	case "gt":
		return actual > threshold
	case "gte":
		return actual >= threshold
	case "lt":
		return actual < threshold
	case "lte":
		return actual <= threshold
	case "eq":
		return actual == threshold
	default:
		return false
	}
}

// ModelSwitchMiddleware returns middleware that routes to different models based on rules.
func ModelSwitchMiddleware(router *ModelRouter, fallback string) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			model, prov, ruleName := router.Route(req)

			if model != "" {
				originalModel := req.Model
				req.Model = model
				if prov != "" {
					req.Provider = prov
				}
				log.Printf("modelswitch: routed %q → %q (rule: %s)", originalModel, model, ruleName)

				if req.Extra == nil {
					req.Extra = make(map[string]any)
				}
				req.Extra["_route_rule"] = ruleName
				req.Extra["_original_model"] = originalModel
			} else if fallback != "" && req.Model == "" {
				req.Model = fallback
			}

			resp, err := next(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("modelswitch: %w", err)
			}

			return resp, nil
		}
	}
}
