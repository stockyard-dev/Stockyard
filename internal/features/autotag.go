package features

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// AutoTagger applies automatic tags to requests based on prompt content patterns.
type AutoTagger struct {
	conn     *sql.DB
	mu       sync.RWMutex
	rules    []AutoTagRule
	compiled []*regexp.Regexp
	lastLoad time.Time
}

// AutoTagRule defines a single auto-tagging rule.
type AutoTagRule struct {
	Tag      string           `json:"tag"`
	Patterns []AutoTagPattern `json:"patterns"`
}

// AutoTagPattern defines a pattern within a rule.
type AutoTagPattern struct {
	Match string `json:"match"`
	Value string `json:"value"`
	Type  string `json:"type"` // "regex" or "contains"
}

// NewAutoTagger creates an auto-tagger that reads rules from proxy_modules config.
func NewAutoTagger(conn *sql.DB) *AutoTagger {
	at := &AutoTagger{conn: conn}
	at.loadRules()
	return at
}

func (at *AutoTagger) loadRules() {
	at.mu.Lock()
	defer at.mu.Unlock()

	if time.Since(at.lastLoad) < 60*time.Second && len(at.rules) > 0 {
		return
	}

	var configJSON sql.NullString
	err := at.conn.QueryRow(`SELECT config_json FROM proxy_modules WHERE name = 'autotag'`).Scan(&configJSON)
	if err != nil || !configJSON.Valid || configJSON.String == "" {
		at.lastLoad = time.Now()
		return
	}

	var cfg struct {
		Rules []AutoTagRule `json:"rules"`
	}
	if err := json.Unmarshal([]byte(configJSON.String), &cfg); err != nil {
		log.Printf("[autotag] config parse error: %v", err)
		at.lastLoad = time.Now()
		return
	}

	at.rules = cfg.Rules

	// Pre-compile regex patterns
	at.compiled = make([]*regexp.Regexp, 0)
	for _, rule := range at.rules {
		for _, p := range rule.Patterns {
			if p.Type == "regex" {
				re, err := regexp.Compile("(?i)" + p.Match)
				if err != nil {
					log.Printf("[autotag] invalid regex %q: %v", p.Match, err)
					at.compiled = append(at.compiled, nil)
				} else {
					at.compiled = append(at.compiled, re)
				}
			} else {
				at.compiled = append(at.compiled, nil)
			}
		}
	}

	at.lastLoad = time.Now()
}

// AutoTagMiddleware returns a middleware that applies auto-tags to requests.
func AutoTagMiddleware(at *AutoTagger) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			at.loadRules()

			at.mu.RLock()
			rules := at.rules
			compiled := at.compiled
			at.mu.RUnlock()

			if len(rules) == 0 {
				return next(ctx, req)
			}

			// Build prompt text from messages
			var sb strings.Builder
			for _, m := range req.Messages {
				sb.WriteString(m.Content)
				sb.WriteString(" ")
			}
			text := sb.String()

			if req.Tags == nil {
				req.Tags = make(map[string]string)
			}

			compIdx := 0
			for _, rule := range rules {
				// Skip if user already set this tag
				if _, exists := req.Tags[rule.Tag]; exists {
					compIdx += len(rule.Patterns)
					continue
				}

				for _, p := range rule.Patterns {
					matched := false
					if p.Type == "regex" {
						if compIdx < len(compiled) && compiled[compIdx] != nil {
							matched = compiled[compIdx].MatchString(text)
						}
					} else {
						matched = strings.Contains(strings.ToLower(text), strings.ToLower(p.Match))
					}
					compIdx++

					if matched {
						req.Tags[rule.Tag] = p.Value
						break // First match wins for this tag
					}
				}
			}

			return next(ctx, req)
		}
	}
}
