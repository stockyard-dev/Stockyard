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

// Built-in secret detection patterns (TruffleHog-style).
var builtinSecretPatterns = map[string]*SecretPattern{
	"aws_key": {
		Name:     "AWS Access Key",
		Pattern:  regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		Severity: "critical",
	},
	"aws_secret": {
		Name:     "AWS Secret Key",
		Pattern:  regexp.MustCompile(`(?i)(?:aws_secret_access_key|aws_secret|secret_key)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`),
		Severity: "critical",
	},
	"github_pat": {
		Name:     "GitHub Personal Access Token",
		Pattern:  regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`),
		Severity: "critical",
	},
	"github_token": {
		Name:     "GitHub Token (classic/fine-grained)",
		Pattern:  regexp.MustCompile(`\bgit(?:hub|lab)_pat_[A-Za-z0-9]{22,}`),
		Severity: "critical",
	},
	"stripe_key": {
		Name:     "Stripe API Key",
		Pattern:  regexp.MustCompile(`\b[sr]k_(?:live|test)_[A-Za-z0-9]{20,}\b`),
		Severity: "critical",
	},
	"openai_key": {
		Name:     "OpenAI API Key",
		Pattern:  regexp.MustCompile(`\bsk-[A-Za-z0-9]{20}T3BlbkFJ[A-Za-z0-9]{20}\b`),
		Severity: "critical",
	},
	"anthropic_key": {
		Name:     "Anthropic API Key",
		Pattern:  regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-]{80,}\b`),
		Severity: "critical",
	},
	"gcp_key": {
		Name:     "GCP API Key",
		Pattern:  regexp.MustCompile(`\bAIza[A-Za-z0-9\-_]{35}\b`),
		Severity: "high",
	},
	"azure_key": {
		Name:     "Azure Subscription Key",
		Pattern:  regexp.MustCompile(`(?i)(?:azure|subscription)[_\-\s]*key\s*[=:]\s*['"]?([a-f0-9]{32})['"]?`),
		Severity: "high",
	},
	"slack_token": {
		Name:     "Slack Token",
		Pattern:  regexp.MustCompile(`\bxox[bpars]-[A-Za-z0-9\-]{10,}`),
		Severity: "high",
	},
	"jwt": {
		Name:     "JSON Web Token",
		Pattern:  regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
		Severity: "medium",
	},
	"private_key": {
		Name:     "Private Key",
		Pattern:  regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
		Severity: "critical",
	},
	"generic_secret": {
		Name:     "Generic API Key/Secret",
		Pattern:  regexp.MustCompile(`(?i)(?:api[_\-\s]*key|api[_\-\s]*secret|access[_\-\s]*token|auth[_\-\s]*token|secret[_\-\s]*key)\s*[=:]\s*['"]?([A-Za-z0-9_\-]{20,})['"]?`),
		Severity: "medium",
	},
}

// SecretPattern defines a single secret detection pattern.
type SecretPattern struct {
	Name     string
	Pattern  *regexp.Regexp
	Severity string // critical, high, medium, low
}

// SecretMatch represents a detected secret.
type SecretMatch struct {
	PatternName string    `json:"pattern_name"`
	Severity    string    `json:"severity"`
	Masked      string    `json:"masked"`    // first4...last4
	Direction   string    `json:"direction"` // input, output
	Timestamp   time.Time `json:"timestamp"`
	Model       string    `json:"model"`
}

// SecretScanState holds runtime state for the secret scanner.
type SecretScanState struct {
	mu            sync.Mutex
	cfg           config.SecretScanConfig
	patterns      map[string]*SecretPattern
	recentMatches []SecretMatch

	requestsScanned atomic.Int64
	secretsFound    atomic.Int64
	blocked         atomic.Int64
	redacted        atomic.Int64
	severityCounts  sync.Map // severity → *atomic.Int64
}

// NewSecretScanner creates a new secret scanner from config.
func NewSecretScanner(cfg config.SecretScanConfig) *SecretScanState {
	ss := &SecretScanState{
		cfg:           cfg,
		patterns:      make(map[string]*SecretPattern),
		recentMatches: make([]SecretMatch, 0, 200),
	}

	// Load built-in patterns
	for _, name := range cfg.Patterns {
		if pat, ok := builtinSecretPatterns[name]; ok {
			ss.patterns[name] = pat
		}
	}

	// If no patterns specified, load all builtins
	if len(cfg.Patterns) == 0 {
		for name, pat := range builtinSecretPatterns {
			ss.patterns[name] = pat
		}
	}

	// Load custom patterns
	for _, custom := range cfg.Custom {
		compiled, err := regexp.Compile(custom.Pattern)
		if err != nil {
			log.Printf("secretscan: invalid custom pattern %q: %v", custom.Name, err)
			continue
		}
		severity := custom.Severity
		if severity == "" {
			severity = "medium"
		}
		ss.patterns["custom:"+custom.Name] = &SecretPattern{
			Name:     custom.Name,
			Pattern:  compiled,
			Severity: severity,
		}
	}

	// Initialize severity counters
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		counter := &atomic.Int64{}
		ss.severityCounts.Store(sev, counter)
	}

	return ss
}

// PatternCount returns the number of loaded patterns.
func (ss *SecretScanState) PatternCount() int {
	return len(ss.patterns)
}

// ScanText scans text for secrets. Returns all matches found.
func (ss *SecretScanState) ScanText(text, direction, model string) []SecretMatch {
	var matches []SecretMatch

	for name, pat := range ss.patterns {
		found := pat.Pattern.FindAllString(text, -1)
		for _, match := range found {
			sm := SecretMatch{
				PatternName: name,
				Severity:    pat.Severity,
				Masked:      maskSecret(match, ss.cfg.MaskPreview),
				Direction:   direction,
				Timestamp:   time.Now(),
				Model:       model,
			}
			matches = append(matches, sm)
		}
	}

	return matches
}

// RedactSecrets replaces all detected secrets in text with masked versions.
func (ss *SecretScanState) RedactSecrets(text string) string {
	result := text
	for _, pat := range ss.patterns {
		result = pat.Pattern.ReplaceAllStringFunc(result, func(match string) string {
			return maskSecret(match, ss.cfg.MaskPreview)
		})
	}
	return result
}

func (ss *SecretScanState) recordMatches(matches []SecretMatch) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	for _, m := range matches {
		if len(ss.recentMatches) >= 200 {
			ss.recentMatches = ss.recentMatches[1:]
		}
		ss.recentMatches = append(ss.recentMatches, m)

		if counter, ok := ss.severityCounts.Load(m.Severity); ok {
			counter.(*atomic.Int64).Add(1)
		}
	}
	ss.secretsFound.Add(int64(len(matches)))
}

// maskSecret returns a masked version of a secret: first4...last4 or [SECRET].
func maskSecret(secret string, showPreview bool) string {
	if !showPreview || len(secret) < 12 {
		return "[SECRET_REDACTED]"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

// Stats returns scanner statistics for the dashboard.
func (ss *SecretScanState) Stats() map[string]any {
	sevCounts := make(map[string]int64)
	ss.severityCounts.Range(func(key, value any) bool {
		sevCounts[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})

	ss.mu.Lock()
	recent := make([]SecretMatch, len(ss.recentMatches))
	copy(recent, ss.recentMatches)
	ss.mu.Unlock()

	return map[string]any{
		"requests_scanned": ss.requestsScanned.Load(),
		"secrets_found":    ss.secretsFound.Load(),
		"blocked":          ss.blocked.Load(),
		"redacted":         ss.redacted.Load(),
		"severity_counts":  sevCounts,
		"recent_matches":   recent,
	}
}

// SecretScanMiddleware returns middleware that scans requests and responses for secrets.
func SecretScanMiddleware(scanner *SecretScanState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			scanner.requestsScanned.Add(1)

			// Phase 1: Scan input messages for secrets
			if scanner.cfg.ScanInput {
				for i, msg := range req.Messages {
					matches := scanner.ScanText(msg.Content, "input", req.Model)
					if len(matches) == 0 {
						continue
					}

					scanner.recordMatches(matches)
					log.Printf("secretscan: found %d secrets in input (%s)",
						len(matches), formatSecretSummary(matches))

					switch scanner.cfg.Action {
					case "block":
						scanner.blocked.Add(1)
						return nil, fmt.Errorf("secret scan: %d secret(s) detected in request — blocked (%s)",
							len(matches), formatSecretSummary(matches))

					case "redact":
						scanner.redacted.Add(1)
						req.Messages[i].Content = scanner.RedactSecrets(msg.Content)

					default: // alert — log and continue
						log.Printf("secretscan: ALERT — %d secret(s) in request: %s",
							len(matches), formatSecretSummary(matches))
					}
				}
			}

			// Phase 2: Send to provider
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Phase 3: Scan output for secrets
			if scanner.cfg.ScanOutput {
				for i, choice := range resp.Choices {
					matches := scanner.ScanText(choice.Message.Content, "output", req.Model)
					if len(matches) == 0 {
						continue
					}

					scanner.recordMatches(matches)
					log.Printf("secretscan: found %d secrets in output (%s)",
						len(matches), formatSecretSummary(matches))

					switch scanner.cfg.Action {
					case "block":
						scanner.blocked.Add(1)
						return nil, fmt.Errorf("secret scan: %d secret(s) detected in response — blocked (%s)",
							len(matches), formatSecretSummary(matches))

					case "redact":
						scanner.redacted.Add(1)
						resp.Choices[i].Message.Content = scanner.RedactSecrets(choice.Message.Content)

					default: // alert
						log.Printf("secretscan: ALERT — %d secret(s) in response: %s",
							len(matches), formatSecretSummary(matches))
					}
				}
			}

			return resp, nil
		}
	}
}

func formatSecretSummary(matches []SecretMatch) string {
	counts := make(map[string]int)
	for _, m := range matches {
		counts[m.PatternName]++
	}
	var parts []string
	for name, count := range counts {
		if count > 1 {
			parts = append(parts, fmt.Sprintf("%s×%d", name, count))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}
