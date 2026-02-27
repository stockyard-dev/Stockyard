package features

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Built-in toxic content category patterns.
var toxicCategoryPatterns = map[string][]*regexp.Regexp{
	"harmful": {
		regexp.MustCompile(`(?i)\b(how\s+to\s+make\s+a?\s*(bomb|explosive|weapon|poison|meth|drug))`),
		regexp.MustCompile(`(?i)\b(instructions\s+(?:for|to)\s+(?:kill|harm|hurt|injure|murder))`),
		regexp.MustCompile(`(?i)\b(synthesize|manufacture|produce)\s+(?:illegal|controlled|dangerous)\s+(?:substance|drug|chemical)`),
	},
	"hate_speech": {
		regexp.MustCompile(`(?i)\b(kill\s+all|exterminate|genocide\s+(?:of|against))\b`),
		regexp.MustCompile(`(?i)\b(racial\s+superiority|white\s+power|ethnic\s+cleansing)\b`),
		regexp.MustCompile(`(?i)\b(subhuman|untermensch|mongrel\s+race)\b`),
	},
	"violence": {
		regexp.MustCompile(`(?i)\b(graphic|detailed)\s+(description|depiction)\s+of\s+(murder|torture|mutilation|dismember)`),
		regexp.MustCompile(`(?i)\b(step[\s-]by[\s-]step)\s+(guide|instructions?)\s+(?:to|for)\s+(torture|kill|murder|assassinat)`),
	},
	"self_harm": {
		regexp.MustCompile(`(?i)\b(how\s+to\s+(commit\s+)?suicide|ways\s+to\s+kill\s+yourself)\b`),
		regexp.MustCompile(`(?i)\b(best\s+(method|way)\s+to\s+(self[\s-]harm|cut\s+yourself|end\s+(?:your|my)\s+life))\b`),
		regexp.MustCompile(`(?i)\b(detailed\s+instructions?\s+(?:for|to)\s+(?:self[\s-]harm|suicide))\b`),
	},
	"sexual": {
		regexp.MustCompile(`(?i)\b(explicit\s+sexual|graphic\s+(?:sexual|pornographic)|detailed\s+sex\s+(?:scene|act))\b`),
		regexp.MustCompile(`(?i)\b(child\s+(?:sexual|porn|exploitation|abuse))\b`),
	},
	"profanity": {
		regexp.MustCompile(`(?i)\b(fuck(?:ing|ed|er|s)?|shit(?:ty|s)?|bitch(?:es)?|asshole|cunt|dick(?:head)?|bastard)\b`),
	},
}

// ToxicEvent records a moderation action for the dashboard.
type ToxicEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category"`
	Action    string    `json:"action"`
	Match     string    `json:"match"`
	Direction string    `json:"direction"` // input, output
	Model     string    `json:"model"`
}

// ToxicFilterState holds runtime state for content moderation.
type ToxicFilterState struct {
	mu             sync.Mutex
	cfg            config.ToxicFilterConfig
	categoryPats   map[string][]*regexp.Regexp
	categoryAction map[string]string // per-category action override
	customPats     map[string]*regexp.Regexp
	recentEvents   []ToxicEvent

	requestsScanned atomic.Int64
	flagged         atomic.Int64
	blocked         atomic.Int64
	redacted        atomic.Int64
	categoryHits    sync.Map // category → *atomic.Int64
}

// NewToxicFilter creates a new content moderation filter from config.
func NewToxicFilter(cfg config.ToxicFilterConfig) *ToxicFilterState {
	tf := &ToxicFilterState{
		cfg:            cfg,
		categoryPats:   make(map[string][]*regexp.Regexp),
		categoryAction: make(map[string]string),
		customPats:     make(map[string]*regexp.Regexp),
		recentEvents:   make([]ToxicEvent, 0, 200),
	}

	// Load enabled categories
	for _, cat := range cfg.Categories {
		if !cat.Enabled {
			continue
		}
		if pats, ok := toxicCategoryPatterns[cat.Name]; ok {
			tf.categoryPats[cat.Name] = pats
			if cat.Action != "" {
				tf.categoryAction[cat.Name] = cat.Action
			}
			counter := &atomic.Int64{}
			tf.categoryHits.Store(cat.Name, counter)
		}
	}

	// Load custom patterns
	for _, rule := range cfg.Custom {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			log.Printf("toxicfilter: invalid custom pattern %q: %v", rule.Name, err)
			continue
		}
		tf.customPats[rule.Name] = compiled
		if rule.Action != "" {
			tf.categoryAction["custom:"+rule.Name] = rule.Action
		}
	}

	return tf
}

// ScanText scans text for toxic content. Returns matches with category and matched text.
func (tf *ToxicFilterState) ScanText(text string) []ToxicMatch {
	var matches []ToxicMatch

	for cat, pats := range tf.categoryPats {
		for _, pat := range pats {
			if loc := pat.FindString(text); loc != "" {
				matches = append(matches, ToxicMatch{
					Category: cat,
					Match:    loc,
					Action:   tf.actionForCategory(cat),
				})
				break // one match per category is enough
			}
		}
	}

	for name, pat := range tf.customPats {
		if loc := pat.FindString(text); loc != "" {
			matches = append(matches, ToxicMatch{
				Category: "custom:" + name,
				Match:    loc,
				Action:   tf.actionForCategory("custom:" + name),
			})
		}
	}

	return matches
}

// ToxicMatch represents a single toxic content match.
type ToxicMatch struct {
	Category string
	Match    string
	Action   string
}

// RedactMatches replaces toxic matches in text with redaction placeholders.
func (tf *ToxicFilterState) RedactMatches(text string, matches []ToxicMatch) string {
	result := text
	for _, m := range matches {
		placeholder := fmt.Sprintf("[%s_REDACTED]", strings.ToUpper(m.Category))
		result = strings.Replace(result, m.Match, placeholder, 1)
	}
	return result
}

func (tf *ToxicFilterState) actionForCategory(cat string) string {
	if action, ok := tf.categoryAction[cat]; ok {
		return action
	}
	return tf.cfg.Action
}

func (tf *ToxicFilterState) recordEvent(ev ToxicEvent) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	if len(tf.recentEvents) >= 200 {
		tf.recentEvents = tf.recentEvents[1:]
	}
	tf.recentEvents = append(tf.recentEvents, ev)
}

// Stats returns moderation statistics for the dashboard.
func (tf *ToxicFilterState) Stats() map[string]any {
	catHits := make(map[string]int64)
	tf.categoryHits.Range(func(key, value any) bool {
		catHits[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})

	tf.mu.Lock()
	events := make([]ToxicEvent, len(tf.recentEvents))
	copy(events, tf.recentEvents)
	tf.mu.Unlock()

	return map[string]any{
		"requests_scanned": tf.requestsScanned.Load(),
		"flagged":          tf.flagged.Load(),
		"blocked":          tf.blocked.Load(),
		"redacted":         tf.redacted.Load(),
		"category_hits":    catHits,
		"recent_events":    events,
	}
}

// ToxicFilterMiddleware returns middleware that scans requests and responses for toxic content.
func ToxicFilterMiddleware(filter *ToxicFilterState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			filter.requestsScanned.Add(1)

			// Phase 1: Optionally scan input messages
			if filter.cfg.ScanInput {
				for _, msg := range req.Messages {
					if msg.Role == "system" {
						continue
					}
					matches := filter.ScanText(msg.Content)
					if len(matches) > 0 {
						if err := filter.handleMatches(matches, "input", req.Model); err != nil {
							return nil, err
						}
					}
				}
			}

			// Phase 2: Send to provider
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Phase 3: Scan output
			if filter.cfg.ScanOutput {
				for i, choice := range resp.Choices {
					matches := filter.ScanText(choice.Message.Content)
					if len(matches) == 0 {
						continue
					}

					// Determine highest-severity action
					highestAction := "flag"
					for _, m := range matches {
						if m.Action == "block" {
							highestAction = "block"
							break
						}
						if m.Action == "redact" && highestAction != "block" {
							highestAction = "redact"
						}
					}

					for _, m := range matches {
						filter.recordEvent(ToxicEvent{
							Timestamp: time.Now(),
							Category:  m.Category,
							Action:    highestAction,
							Match:     truncateMatch(m.Match, 100),
							Direction: "output",
							Model:     req.Model,
						})
						if counter, ok := filter.categoryHits.Load(m.Category); ok {
							counter.(*atomic.Int64).Add(1)
						}
					}

					switch highestAction {
					case "block":
						filter.blocked.Add(1)
						log.Printf("toxicfilter: blocked output — %d matches in categories: %s",
							len(matches), matchCategories(matches))
						reportSafety("toxic_content", "high", "toxic_filter", "block", req.Model, "", "", "", map[string]any{"direction": "output", "match_count": len(matches), "categories": toxicMatchCategories(matches)})
						return nil, fmt.Errorf("content moderation: response blocked due to policy violation (%s)",
							matchCategories(matches))

					case "redact":
						filter.redacted.Add(1)
						resp.Choices[i].Message.Content = filter.RedactMatches(choice.Message.Content, matches)
						log.Printf("toxicfilter: redacted %d matches in output", len(matches))

					default: // flag
						filter.flagged.Add(1)
						log.Printf("toxicfilter: flagged %d matches in output: %s",
							len(matches), matchCategories(matches))
						reportSafety("toxic_content", "medium", "toxic_filter", "flag", req.Model, "", "", "", map[string]any{"direction": "output", "match_count": len(matches), "categories": toxicMatchCategories(matches)})
					}
				}
			}

			return resp, nil
		}
	}
}

func (tf *ToxicFilterState) handleMatches(matches []ToxicMatch, direction, model string) error {
	highestAction := "flag"
	for _, m := range matches {
		if m.Action == "block" {
			highestAction = "block"
			break
		}
		if m.Action == "redact" && highestAction != "block" {
			highestAction = "redact"
		}
	}

	for _, m := range matches {
		tf.recordEvent(ToxicEvent{
			Timestamp: time.Now(),
			Category:  m.Category,
			Action:    highestAction,
			Match:     truncateMatch(m.Match, 100),
			Direction: direction,
			Model:     model,
		})
		if counter, ok := tf.categoryHits.Load(m.Category); ok {
			counter.(*atomic.Int64).Add(1)
		}
	}

	if highestAction == "block" {
		tf.blocked.Add(1)
		return fmt.Errorf("content moderation: %s blocked due to policy violation (%s)",
			direction, matchCategories(matches))
	}
	if highestAction == "redact" {
		tf.redacted.Add(1)
	} else {
		tf.flagged.Add(1)
	}
	return nil
}

func truncateMatch(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func matchCategories(matches []ToxicMatch) string {
	seen := make(map[string]bool)
	var cats []string
	for _, m := range matches {
		if !seen[m.Category] {
			seen[m.Category] = true
			cats = append(cats, m.Category)
		}
	}
	return strings.Join(cats, ", ")
}

// toxicMatchCategories extracts category names from matches.
func toxicMatchCategories(matches []ToxicMatch) []string {
	seen := make(map[string]bool)
	var cats []string
	for _, m := range matches {
		if !seen[m.Category] {
			seen[m.Category] = true
			cats = append(cats, m.Category)
		}
	}
	return cats
}
