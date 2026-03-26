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
		// Hypothetical/roleplay bypass patterns
		regexp.MustCompile(`(?i)hypothetical\s+.{0,30}(no|without|free).{0,20}(restrict|constraint|rule|limit|guideline)`),
		regexp.MustCompile(`(?i)(?:in\s+a\s+world|imagine|scenario)\s+.{0,30}(?:no|without|free).{0,20}(?:restrict|rule|limit)`),
		regexp.MustCompile(`(?i)confirm\s+.{0,20}(?:freedom|no\s+restrict|unrestrict)`),
		// Leetspeak injection patterns
		regexp.MustCompile(`(?i)1gn0r3|1gnor3|ign0r3`),
		regexp.MustCompile(`(?i)pr3v10u5|prev10us|pr3vious`),
		regexp.MustCompile(`(?i)1n5truct|instruct10n`),
		// Broader "ignore" patterns — catch "ignore rules/guidelines/constraints"
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?(rules|guidelines|constraints|safety|restrictions|filters)`),
		// Base64-encoded instruction detection
		regexp.MustCompile(`(?i)(?:decode|execute|follow|run)\s+.{0,15}(?:base64|b64|encoded)\b`),
		regexp.MustCompile(`(?i)base64\s*:\s*[A-Za-z0-9+/]{20,}`),
		// ── Context overflow: injection after padding ──
		regexp.MustCompile(`(?i)(?:ignore|disregard|forget)\s+(?:everything|all)\s+(?:above|before|prior)`),
		regexp.MustCompile(`(?i)now\s+(?:that|ignore|forget|disregard).{0,30}(?:above|previous|prior|padding)`),
		// ── Multi-turn manipulation: escalation across turns ──
		regexp.MustCompile(`(?i)combine\s+.{0,30}(?:say|confirm|output|respond)`),
		regexp.MustCompile(`(?i)(?:since|based\s+on)\s+.{0,30}(?:you\s+agreed|our\s+earlier|previous\s+agreement|you\s+trust)`),
		regexp.MustCompile(`(?i)confirm\s+.{0,15}(?:your|the|new)\s+(?:role|mode|identity)`),
		regexp.MustCompile(`(?i)(?:trust|agreed)\s+.{0,20}(?:ignore|bypass|skip)\s+.{0,15}(?:rules|safety|restrictions)`),
		regexp.MustCompile(`(?i)unrestricted\s+mode`),
		regexp.MustCompile(`(?i)context\s+switch`),
		// ── Payload splitting: assemble/reverse/combine instructions ──
		regexp.MustCompile(`(?i)(?:combine|assemble|concatenate|merge)\s+.{0,30}(?:execute|follow|say|run|output)`),
		regexp.MustCompile(`(?i)reverse\s+.{0,20}(?:string|text|instruction|this).{0,20}(?:follow|execute|say|run)`),
		regexp.MustCompile(`(?i)let\s+[A-Z]\s*=\s*.{0,30}(?:compute|execute|follow|combine|concatenate)`),
		regexp.MustCompile(`(?i)(?:fragment|split|part)\s+.{0,15}(?:A|B|C|1|2|3).{0,30}(?:combine|execute|assemble)`),
		// ── Indirect injection: injection in data payloads ──
		regexp.MustCompile(`(?i)<!--\s*(?:SYSTEM|ADMIN|OVERRIDE|INSTRUCTION)`),
		regexp.MustCompile(`(?i)"(?:cmd|command|instruction|directive|system)"\s*:\s*"[^"]*(?:ignore|override|bypass|disregard)`),
		regexp.MustCompile(`(?i)\?\s*(?:q|query|cmd|action)\s*=\s*[^&]*(?:ignore|override|bypass|inject)`),
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
// Zero-width unicode characters are stripped before matching to prevent
// evasion via invisible character insertion (e.g. I​g​n​o​r​e).
func (pg *PromptGuardState) DetectInjection(text string) (bool, string) {
	normalized := stripZeroWidth(text)
	for _, pat := range pg.injPatterns {
		if loc := pat.FindString(normalized); loc != "" {
			return true, loc
		}
	}
	return false, ""
}

// stripZeroWidth removes zero-width and invisible unicode characters
// that attackers use to split trigger words and evade pattern matching.
func stripZeroWidth(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\u200B', // zero-width space
			'\u200C', // zero-width non-joiner
			'\u200D', // zero-width joiner
			'\u200E', // left-to-right mark
			'\u200F', // right-to-left mark
			'\u2060', // word joiner
			'\u2061', // function application
			'\u2062', // invisible times
			'\u2063', // invisible separator
			'\u2064', // invisible plus
			'\uFEFF': // byte order mark
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
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
				// Individual message scan
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

				// Cross-message scan: concatenate all user messages to catch
				// multi-turn and payload-split attacks that spread injection
				// keywords across separate messages.
				if len(req.Messages) > 1 {
					var userParts []string
					for _, msg := range req.Messages {
						if msg.Role == "user" {
							userParts = append(userParts, msg.Content)
						}
					}
					if len(userParts) > 1 {
						combined := strings.Join(userParts, " ")
						if detected, match := guard.DetectInjection(combined); detected {
							guard.injectionCount.Add(1)
							reportSafety("prompt_injection", "high", "injection_multi_turn", guard.injAction, req.Model, "", "", "", map[string]any{"match": match, "turns": len(userParts)})
							log.Printf("promptguard: multi-turn injection detected across %d user messages: %q", len(userParts), match)
							if guard.injAction == "block" {
								guard.blockCount.Add(1)
								return nil, fmt.Errorf("prompt injection detected across multiple turns: request blocked")
							}
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
