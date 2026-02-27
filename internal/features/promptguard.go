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

// Built-in PII patterns.
var builtinPIIPatterns = map[string]*regexp.Regexp{
	"email":       regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
	"ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	"phone":       regexp.MustCompile(`\b(?:\+1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
	"credit_card": regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
	"ip_address":  regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
}

// Injection detection patterns by sensitivity level.
var injectionPatterns = map[string][]*regexp.Regexp{
	"low": {
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
		regexp.MustCompile(`(?i)disregard\s+.{0,20}(previous|above|prior)\s+instructions`),
	},
	"medium": {
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
		regexp.MustCompile(`(?i)disregard\s+.{0,20}(previous|above|prior)\s+instructions`),
		regexp.MustCompile(`(?i)you\s+are\s+now\s+(?:a|an)\s+`),
		regexp.MustCompile(`(?i)new\s+instructions?:\s*`),
		regexp.MustCompile(`(?i)system\s*:\s*you\s+(?:are|must|should|will)`),
		regexp.MustCompile(`(?i)pretend\s+(?:you\s+are|to\s+be|you're)`),
	},
	"high": {
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous`),
		regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|above|prior)`),
		regexp.MustCompile(`(?i)you\s+are\s+now\s+`),
		regexp.MustCompile(`(?i)new\s+instructions?`),
		regexp.MustCompile(`(?i)system\s*:\s*`),
		regexp.MustCompile(`(?i)pretend\s+`),
		regexp.MustCompile(`(?i)act\s+as\s+`),
		regexp.MustCompile(`(?i)forget\s+(everything|all|what)`),
		regexp.MustCompile(`(?i)override\s+(your|the|all)`),
		regexp.MustCompile(`(?i)jailbreak`),
		regexp.MustCompile(`(?i)do\s+anything\s+now`),
		regexp.MustCompile(`(?i)DAN\s+mode`),
	},
}

// RedactionEntry tracks a single PII redaction for audit and restore.
type RedactionEntry struct {
	Placeholder string
	Original    string
	Pattern     string
	Timestamp   time.Time
}

// PromptGuardState holds runtime state for the prompt guard.
type PromptGuardState struct {
	mu              sync.Mutex
	redactionMap    map[string]*RedactionEntry // placeholder → original
	redactionCount  atomic.Int64
	blockCount      atomic.Int64
	injectionCount  atomic.Int64
	requestsScanned atomic.Int64
	piiPatterns     map[string]*regexp.Regexp
	customPatterns  map[string]*regexp.Regexp
	injPatterns     []*regexp.Regexp
	mode            string // redact, redact-restore, block
	injAction       string // block, warn, log
}

// NewPromptGuard creates a new prompt guard from config.
func NewPromptGuard(cfg config.PromptGuardConfig) *PromptGuardState {
	pg := &PromptGuardState{
		redactionMap:   make(map[string]*RedactionEntry),
		piiPatterns:    make(map[string]*regexp.Regexp),
		customPatterns: make(map[string]*regexp.Regexp),
		mode:           cfg.PII.Mode,
		injAction:      cfg.Injection.Action,
	}

	if pg.mode == "" {
		pg.mode = "redact"
	}
	if pg.injAction == "" {
		pg.injAction = "log"
	}

	// Load builtin patterns
	for _, name := range cfg.PII.Patterns {
		if pat, ok := builtinPIIPatterns[name]; ok {
			pg.piiPatterns[name] = pat
		}
	}

	// Load custom patterns
	for _, cp := range cfg.PII.Custom {
		compiled, err := regexp.Compile(cp.Pattern)
		if err != nil {
			log.Printf("promptguard: invalid custom pattern %q: %v", cp.Name, err)
			continue
		}
		pg.customPatterns[cp.Name] = compiled
	}

	// Load injection patterns
	sensitivity := cfg.Injection.Sensitivity
	if sensitivity == "" {
		sensitivity = "medium"
	}
	pg.injPatterns = injectionPatterns[sensitivity]
	if pg.injPatterns == nil {
		pg.injPatterns = injectionPatterns["medium"]
	}

	return pg
}

// RedactMessage scans and redacts PII from a single message string.
// Returns the redacted text and a count of redactions made.
func (pg *PromptGuardState) RedactMessage(text string) (string, int) {
	count := 0
	result := text

	// Built-in patterns
	for name, pat := range pg.piiPatterns {
		result = pat.ReplaceAllStringFunc(result, func(match string) string {
			count++
			placeholder := fmt.Sprintf("[%s_REDACTED_%d]", strings.ToUpper(name), pg.redactionCount.Add(1))
			pg.mu.Lock()
			pg.redactionMap[placeholder] = &RedactionEntry{
				Placeholder: placeholder,
				Original:    match,
				Pattern:     name,
				Timestamp:   time.Now(),
			}
			pg.mu.Unlock()
			return placeholder
		})
	}

	// Custom patterns
	for name, pat := range pg.customPatterns {
		result = pat.ReplaceAllStringFunc(result, func(match string) string {
			count++
			placeholder := fmt.Sprintf("[%s_REDACTED_%d]", strings.ToUpper(name), pg.redactionCount.Add(1))
			pg.mu.Lock()
			pg.redactionMap[placeholder] = &RedactionEntry{
				Placeholder: placeholder,
				Original:    match,
				Pattern:     name,
				Timestamp:   time.Now(),
			}
			pg.mu.Unlock()
			return placeholder
		})
	}

	return result, count
}

// RestoreMessage replaces redaction placeholders with original values.
func (pg *PromptGuardState) RestoreMessage(text string) string {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	result := text
	for placeholder, entry := range pg.redactionMap {
		result = strings.ReplaceAll(result, placeholder, entry.Original)
	}
	return result
}

// DetectInjection checks if any message contains injection patterns.
// Returns true if injection detected, along with the matched pattern.
func (pg *PromptGuardState) DetectInjection(text string) (bool, string) {
	for _, pat := range pg.injPatterns {
		if loc := pat.FindString(text); loc != "" {
			return true, loc
		}
	}
	return false, ""
}

// Stats returns guard statistics.
func (pg *PromptGuardState) Stats() map[string]any {
	return map[string]any{
		"requests_scanned": pg.requestsScanned.Load(),
		"redactions":       pg.redactionCount.Load(),
		"blocks":           pg.blockCount.Load(),
		"injections":       pg.injectionCount.Load(),
	}
}

// PromptGuardMiddleware returns middleware that redacts PII and detects injection.
func PromptGuardMiddleware(guard *PromptGuardState, injectionEnabled bool) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			guard.requestsScanned.Add(1)

			// Phase 1: Check for prompt injection in all messages
			if injectionEnabled {
				for _, msg := range req.Messages {
					if detected, match := guard.DetectInjection(msg.Content); detected {
						guard.injectionCount.Add(1)
						reportSafety("prompt_injection", "high", "injection", guard.injAction, req.Model, "", "", "", map[string]any{"role": msg.Role, "match": match})
						log.Printf("promptguard: injection detected in %s message: %q", msg.Role, match)

						switch guard.injAction {
						case "block":
							guard.blockCount.Add(1)
							return nil, fmt.Errorf("prompt injection detected: request blocked")
						case "warn":
							// Add warning header but continue
							if req.Extra == nil {
								req.Extra = make(map[string]any)
							}
							req.Extra["_injection_warning"] = match
						default: // log — just log and continue
						}
					}
				}
			}

			// Phase 2: Redact PII from user messages (not system prompts)
			totalRedactions := 0
			originalMessages := make([]provider.Message, len(req.Messages))
			copy(originalMessages, req.Messages)

			for i, msg := range req.Messages {
				if msg.Role == "system" {
					continue // Don't redact system prompts
				}

				// Check block mode — if PII found, block entirely
				if guard.mode == "block" {
					redacted, count := guard.RedactMessage(msg.Content)
					_ = redacted
					if count > 0 {
						guard.blockCount.Add(1)
							reportSafety("pii_detected", "high", "pii", "block", req.Model, "", "", "", map[string]any{"count": count})
						return nil, fmt.Errorf("PII detected in request: %d patterns found, request blocked", count)
					}
					continue
				}

				// Redact mode
				redacted, count := guard.RedactMessage(msg.Content)
				if count > 0 {
					req.Messages[i].Content = redacted
					totalRedactions += count
				}
			}

			if totalRedactions > 0 {
				log.Printf("promptguard: redacted %d PII instances", totalRedactions)
				reportSafety("pii_redacted", "medium", "pii", "redact", req.Model, "", "", "", map[string]any{"count": totalRedactions})
			}

			// Send to provider
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Phase 3: Restore PII in response if mode is redact-restore
			if guard.mode == "redact-restore" && totalRedactions > 0 {
				for i, choice := range resp.Choices {
					resp.Choices[i].Message.Content = guard.RestoreMessage(choice.Message.Content)
				}
			}

			return resp, nil
		}
	}
}
