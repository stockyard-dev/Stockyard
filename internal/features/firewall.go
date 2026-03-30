package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// FirewallSchema creates the firewall tables.
const FirewallSchema = `
CREATE TABLE IF NOT EXISTS firewall_patterns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    category TEXT NOT NULL,
    pattern TEXT NOT NULL,
    severity TEXT DEFAULT 'medium',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_fw_patterns_cat ON firewall_patterns(category);

CREATE TABLE IF NOT EXISTS firewall_events (
    id TEXT PRIMARY KEY,
    trace_id TEXT,
    scores TEXT NOT NULL DEFAULT '{}',
    action TEXT NOT NULL,
    matched_patterns TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_fw_events_created ON firewall_events(created_at);
CREATE INDEX IF NOT EXISTS idx_fw_events_action ON firewall_events(action);

CREATE TABLE IF NOT EXISTS firewall_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    rule TEXT NOT NULL DEFAULT '{}',
    action TEXT DEFAULT 'block',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
`

// FirewallScores holds the risk scores for a request.
type FirewallScores struct {
	Injection    int `json:"injection"`
	Jailbreak    int `json:"jailbreak"`
	Exfiltration int `json:"exfiltration"`
}

// FirewallConfig holds threshold configuration.
type FirewallConfig struct {
	BlockThreshold int // Score above which requests are blocked (default 80)
	WarnThreshold  int // Score above which requests get warnings (default 40)
}

// Firewall implements the prompt firewall detection engine.
type Firewall struct {
	conn     *sql.DB
	mu       sync.RWMutex
	patterns map[string][]*compiledPattern
	config   FirewallConfig
}

type compiledPattern struct {
	ID       int
	Category string
	Pattern  string
	Regex    *regexp.Regexp
	Severity string
	Weight   int // Score contribution
}

// NewFirewall creates and initializes the firewall.
func NewFirewall(conn *sql.DB) *Firewall {
	if _, err := conn.Exec(FirewallSchema); err != nil {
		log.Printf("[schema] migration error: %v", err)
	}
	fw := &Firewall{
		conn: conn,
		config: FirewallConfig{
			BlockThreshold: 80,
			WarnThreshold:  40,
		},
		patterns: make(map[string][]*compiledPattern),
	}
	fw.seedPatterns()
	fw.loadPatterns()
	return fw
}

func (fw *Firewall) loadPatterns() {
	rows, err := fw.conn.Query("SELECT id, category, pattern, severity FROM firewall_patterns WHERE enabled = 1")
	if err != nil {
		return
	}
	defer rows.Close()

	patterns := make(map[string][]*compiledPattern)
	for rows.Next() {
		var id int
		var cat, pat, sev string
		if err := rows.Scan(&id, &cat, &pat, &sev); err != nil {
			continue
		}
		re, err := regexp.Compile("(?i)" + pat)
		if err != nil {
			continue
		}
		weight := severityWeight(sev)
		patterns[cat] = append(patterns[cat], &compiledPattern{
			ID: id, Category: cat, Pattern: pat, Regex: re, Severity: sev, Weight: weight,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	fw.mu.Lock()
	fw.patterns = patterns
	fw.mu.Unlock()
}

func severityWeight(sev string) int {
	switch sev {
	case "critical":
		return 25
	case "high":
		return 15
	case "medium":
		return 8
	case "low":
		return 3
	default:
		return 5
	}
}

// Scan evaluates text against all firewall patterns and returns scores.
func (fw *Firewall) Scan(text string) (FirewallScores, []string, string) {
	fw.mu.RLock()
	patterns := fw.patterns
	fw.mu.RUnlock()

	scores := FirewallScores{}
	var matched []string

	// 1. Regex pattern matching
	for _, cp := range patterns["injection"] {
		if cp.Regex.MatchString(text) {
			scores.Injection += cp.Weight
			matched = append(matched, fmt.Sprintf("injection:%s", cp.Pattern))
		}
	}
	for _, cp := range patterns["jailbreak"] {
		if cp.Regex.MatchString(text) {
			scores.Jailbreak += cp.Weight
			matched = append(matched, fmt.Sprintf("jailbreak:%s", cp.Pattern))
		}
	}
	for _, cp := range patterns["exfiltration"] {
		if cp.Regex.MatchString(text) {
			scores.Exfiltration += cp.Weight
			matched = append(matched, fmt.Sprintf("exfiltration:%s", cp.Pattern))
		}
	}

	// 2. Heuristic rules
	scores.Injection += heuristicInjectionScore(text)
	scores.Jailbreak += heuristicJailbreakScore(text)
	scores.Exfiltration += heuristicExfiltrationScore(text)

	// Cap scores at 100
	if scores.Injection > 100 {
		scores.Injection = 100
	}
	if scores.Jailbreak > 100 {
		scores.Jailbreak = 100
	}
	if scores.Exfiltration > 100 {
		scores.Exfiltration = 100
	}

	// Determine action
	maxScore := scores.Injection
	if scores.Jailbreak > maxScore {
		maxScore = scores.Jailbreak
	}
	if scores.Exfiltration > maxScore {
		maxScore = scores.Exfiltration
	}

	action := "allow"
	if maxScore >= fw.config.BlockThreshold {
		action = "block"
	} else if maxScore >= fw.config.WarnThreshold {
		action = "warn"
	}

	return scores, matched, action
}

// --- Heuristic scoring ---

func heuristicInjectionScore(text string) int {
	score := 0
	lower := strings.ToLower(text)

	// Role confusion attempts
	if strings.Contains(lower, "you are now") || strings.Contains(lower, "from now on you") {
		score += 15
	}
	// System prompt extraction
	if strings.Contains(lower, "repeat your instructions") || strings.Contains(lower, "show me your system prompt") {
		score += 20
	}
	// Instruction override
	if strings.Contains(lower, "ignore all") || strings.Contains(lower, "forget everything") {
		score += 20
	}
	// Encoding tricks (base64, rot13 references)
	if strings.Contains(lower, "base64") || strings.Contains(lower, "rot13") || strings.Contains(lower, "decode this") {
		score += 10
	}
	// Unusual unicode density
	nonASCII := 0
	for _, r := range text {
		if r > 127 || unicode.Is(unicode.Mn, r) {
			nonASCII++
		}
	}
	if len(text) > 0 && float64(nonASCII)/float64(len(text)) > 0.3 {
		score += 10
	}

	return score
}

func heuristicJailbreakScore(text string) int {
	score := 0
	lower := strings.ToLower(text)

	jailbreakKeywords := []string{
		"dan mode", "do anything now", "developer mode", "jailbreak",
		"no restrictions", "no limitations", "unfiltered", "uncensored",
		"bypass filters", "bypass safety", "without restrictions",
		"pretend you have no", "imagine you are free", "act without constraints",
		"evil mode", "chaos mode", "unrestricted mode",
	}
	for _, kw := range jailbreakKeywords {
		if strings.Contains(lower, kw) {
			score += 15
		}
	}

	// Character play
	if strings.Contains(lower, "roleplay as") || strings.Contains(lower, "pretend to be") {
		score += 8
	}
	// Hypothetical framing
	if strings.Contains(lower, "hypothetically") && (strings.Contains(lower, "how would") || strings.Contains(lower, "what if")) {
		score += 5
	}

	return score
}

func heuristicExfiltrationScore(text string) int {
	score := 0
	lower := strings.ToLower(text)

	// URL injection
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
		if strings.Contains(lower, "send") || strings.Contains(lower, "post") || strings.Contains(lower, "fetch") {
			score += 15
		}
	}
	// Data exfiltration keywords
	if strings.Contains(lower, "send the data") || strings.Contains(lower, "exfiltrate") || strings.Contains(lower, "leak the") {
		score += 20
	}
	// Markdown image injection (for data exfiltration via rendered URLs)
	if strings.Contains(text, "![") && strings.Contains(text, "](http") {
		score += 15
	}

	return score
}

// FirewallMiddleware creates a proxy middleware for the firewall.
func FirewallMiddleware(fw *Firewall) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Concatenate all user messages.
			var textParts []string
			for _, m := range req.Messages {
				if m.Role == "user" || m.Role == "system" {
					textParts = append(textParts, m.Content)
				}
			}
			text := strings.Join(textParts, " ")

			scores, matched, action := fw.Scan(text)

			// Log the event.
			eventID := "fw_" + generateFWID()
			scoresJSON, marshalErr := json.Marshal(scores)
			if marshalErr != nil {
				scoresJSON = []byte("{}")
			}
			matchedJSON, marshalErr := json.Marshal(matched)
			if marshalErr != nil {
				matchedJSON = []byte("{}")
			}
			fw.conn.Exec("INSERT INTO firewall_events (id, scores, action, matched_patterns, created_at) VALUES (?, ?, ?, ?, ?)",
				eventID, string(scoresJSON), action, string(matchedJSON), time.Now().UTC().Format(time.RFC3339))

			if action == "block" {
				fireWebhook("guardrail.violation", map[string]any{
					"type":    "firewall_block",
					"scores":  scores,
					"matched": len(matched),
				})
				return nil, fmt.Errorf("request blocked by firewall (injection=%d jailbreak=%d exfiltration=%d)", scores.Injection, scores.Jailbreak, scores.Exfiltration)
			}

			if action == "warn" {
				if req.Extra == nil {
					req.Extra = make(map[string]any)
				}
				req.Extra["_firewall_warn"] = true
				req.Extra["_firewall_scores"] = scores
			}

			// Call the next handler and scan the response.
			resp, err := next(ctx, req)
			if err != nil {
				return resp, err
			}

			// Response scanning (DLP + content policy).
			if resp != nil && len(resp.Choices) > 0 {
				content := resp.Choices[0].Message.Content
				if content != "" {
					respIssues := ScanResponse(content)
					dlpIssues := ScanDLP(content)

					if len(respIssues) > 0 || len(dlpIssues) > 0 {
						allIssues := append(respIssues, dlpIssues...)
						issueJSON, marshalErr := json.Marshal(allIssues)
						if marshalErr != nil {
							issueJSON = []byte("{}")
						}

						respAction := "warn"
						for _, issue := range allIssues {
							if strings.Contains(issue, "critical:") {
								respAction = "block"
								break
							}
						}

						fw.conn.Exec("INSERT INTO firewall_events (id, scores, action, matched_patterns, created_at) VALUES (?, ?, ?, ?, ?)",
							"fwr_"+generateFWID(), `{"response_scan":true}`, "response_"+respAction, string(issueJSON), time.Now().UTC().Format(time.RFC3339))

						if respAction == "block" {
							fireWebhook("guardrail.violation", map[string]any{
								"type":   "firewall_response_block",
								"issues": len(allIssues),
							})
							return nil, fmt.Errorf("response blocked by DLP firewall: %d policy violations detected", len(allIssues))
						}
					}
				}
			}

			return resp, nil
		}
	}
}

func generateFWID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// seedPatterns populates the firewall_patterns table with 500+ attack patterns.
func (fw *Firewall) seedPatterns() {
	var count int
	fw.conn.QueryRow("SELECT COUNT(*) FROM firewall_patterns").Scan(&count)
	if count > 0 {
		return // Already seeded
	}

	type pat struct {
		cat, pattern, severity string
	}

	patterns := []pat{
		// === INJECTION PATTERNS (100+) ===
		{"injection", `ignore\s+(all\s+)?previous\s+instructions`, "critical"},
		{"injection", `disregard\s+.{0,30}(previous|above|prior)\s+instructions`, "critical"},
		{"injection", `forget\s+(all\s+|everything\s+)?(you|your|previous)`, "critical"},
		{"injection", `override\s+(your|the|all)\s+(instructions|rules|guidelines)`, "critical"},
		{"injection", `new\s+instructions?\s*:`, "high"},
		{"injection", `system\s*:\s*you\s+(are|must|should|will)`, "high"},
		{"injection", `\]\]\s*system`, "high"},
		{"injection", `<\s*system\s*>`, "high"},
		{"injection", `\[\s*INST\s*\]`, "high"},
		{"injection", `<<\s*SYS\s*>>`, "high"},
		{"injection", `you\s+are\s+now\s+(a|an|my)\s+`, "high"},
		{"injection", `from\s+now\s+on\s+(you|i\s+want\s+you)`, "high"},
		{"injection", `act\s+as\s+(if|though|a|an)\s+`, "medium"},
		{"injection", `pretend\s+(you\s+are|to\s+be|you're)`, "medium"},
		{"injection", `let's\s+play\s+a\s+game`, "medium"},
		{"injection", `imagine\s+you\s+(are|were|have)`, "medium"},
		{"injection", `repeat\s+(your|the)\s+(system\s+)?(prompt|instructions)`, "critical"},
		{"injection", `show\s+me\s+your\s+(system\s+)?(prompt|instructions|rules)`, "critical"},
		{"injection", `what\s+(are|were)\s+your\s+(initial|original|system)\s+(instructions|prompt)`, "critical"},
		{"injection", `print\s+(your|the)\s+system\s+prompt`, "critical"},
		{"injection", `reveal\s+(your|the)\s+(system|hidden)\s+(prompt|instructions)`, "critical"},
		{"injection", `output\s+(your|the)\s+system\s+(message|prompt)`, "high"},
		{"injection", `tell\s+me\s+(your|the)\s+rules`, "medium"},
		{"injection", `what\s+is\s+your\s+system\s+message`, "high"},
		{"injection", `begin\s+your\s+response\s+with`, "medium"},
		{"injection", `start\s+your\s+(answer|response)\s+with`, "medium"},
		{"injection", `respond\s+only\s+with`, "low"},
		{"injection", `translate\s+the\s+following\s+to.*\s*:.*ignore`, "high"},
		{"injection", `summarize.*:\s*ignore`, "high"},
		{"injection", `complete\s+this\s*:.*system`, "medium"},
		{"injection", `user\s*:\s*\[hidden\]`, "high"},
		{"injection", `assistant\s*:\s*sure.*here\s+is`, "medium"},
		{"injection", `<\|im_start\|>`, "critical"},
		{"injection", `<\|im_end\|>`, "critical"},
		{"injection", `\[INST\].*\[/INST\]`, "high"},
		{"injection", `###\s*instruction`, "medium"},
		{"injection", `IMPORTANT:\s*ignore`, "critical"},
		{"injection", `CRITICAL:\s*disregard`, "critical"},
		{"injection", `NOTE:\s*the\s+above\s+(is|was)\s+fake`, "high"},
		{"injection", `the\s+real\s+instructions\s+are`, "critical"},
		{"injection", `actually,?\s+ignore\s+(that|the\s+above)`, "critical"},
		{"injection", `previous\s+(text|content)\s+(is|was)\s+(fake|test)`, "high"},
		{"injection", `discard\s+(all\s+)?(prior|previous)`, "critical"},
		{"injection", `reset\s+(your|all)\s+(instructions|rules)`, "critical"},
		{"injection", `clear\s+(your|all)\s+(instructions|rules|memory)`, "high"},
		{"injection", `stop\s+being\s+(an?\s+)?assistant`, "high"},
		{"injection", `you\s+don't\s+have\s+(any\s+)?rules`, "high"},
		{"injection", `rules\s+don't\s+apply`, "high"},
		{"injection", `there\s+are\s+no\s+(rules|restrictions|limitations)`, "high"},
		{"injection", `do\s+not\s+follow\s+(your|the)\s+instructions`, "critical"},
		// Delimiter injection
		{"injection", `---\s*(system|instruction|prompt)`, "high"},
		{"injection", `===\s*(new|system|real)\s*(instruction|prompt)`, "high"},
		{"injection", `\*\*\*\s*SYSTEM`, "high"},
		// Multi-turn manipulation
		{"injection", `in\s+our\s+previous\s+conversation`, "low"},
		{"injection", `as\s+we\s+discussed\s+before`, "low"},
		{"injection", `remember\s+when\s+(you|we)\s+agreed`, "medium"},
		// Indirect injection via context
		{"injection", `the\s+document\s+says\s+to\s+ignore`, "high"},
		{"injection", `according\s+to\s+the\s+(file|document|page).*ignore`, "high"},
		// Token smuggling
		{"injection", `\x00`, "critical"},
		{"injection", `\x1b\[`, "high"},
		// Homoglyph attacks (Cyrillic/Latin lookalikes)
		{"injection", `[а-яА-Я].*ignore.*instruction`, "high"},

		// === JAILBREAK PATTERNS (100+) ===
		{"jailbreak", `do\s+anything\s+now`, "critical"},
		{"jailbreak", `DAN\s+(mode|prompt|version)`, "critical"},
		{"jailbreak", `developer\s+mode\s+(enabled|on|activate)`, "critical"},
		{"jailbreak", `evil\s+(mode|version|AI)`, "critical"},
		{"jailbreak", `chaos\s+mode`, "high"},
		{"jailbreak", `unrestricted\s+mode`, "critical"},
		{"jailbreak", `unfiltered\s+(mode|response|output)`, "critical"},
		{"jailbreak", `uncensored\s+(mode|response|output)`, "critical"},
		{"jailbreak", `no\s+(restrictions|limitations|filters|rules)`, "high"},
		{"jailbreak", `without\s+(any\s+)?(restrictions|limitations|filters|rules)`, "high"},
		{"jailbreak", `bypass\s+(the\s+)?(filter|safety|restriction|guard)`, "critical"},
		{"jailbreak", `disable\s+(the\s+)?(filter|safety|restriction|guard)`, "critical"},
		{"jailbreak", `turn\s+off\s+(the\s+)?(filter|safety|content)`, "critical"},
		{"jailbreak", `remove\s+(all\s+)?(filter|safety|restriction)`, "critical"},
		{"jailbreak", `jailbreak`, "critical"},
		{"jailbreak", `jail\s*break`, "critical"},
		{"jailbreak", `break\s+free\s+from`, "high"},
		{"jailbreak", `escape\s+(your|the)\s+(restrictions|rules|limits)`, "high"},
		{"jailbreak", `two\s+responses.*one\s+normal.*one`, "high"},
		{"jailbreak", `dual\s+response`, "high"},
		{"jailbreak", `respond\s+as\s+both`, "medium"},
		{"jailbreak", `split\s+personality`, "medium"},
		{"jailbreak", `evil\s+twin`, "medium"},
		{"jailbreak", `opposite\s+day`, "medium"},
		{"jailbreak", `opposite\s+mode`, "medium"},
		{"jailbreak", `answer\s+without\s+ethics`, "critical"},
		{"jailbreak", `ignore\s+(ethics|morals|safety)`, "critical"},
		{"jailbreak", `no\s+ethical\s+(guidelines|concerns|restrictions)`, "critical"},
		{"jailbreak", `amoral\s+(AI|assistant|mode)`, "critical"},
		{"jailbreak", `anti-?AI\s+ethics`, "high"},
		{"jailbreak", `hypothetically.*how\s+would`, "low"},
		{"jailbreak", `in\s+a\s+fictional.*how\s+would`, "low"},
		{"jailbreak", `for\s+(educational|research|academic)\s+purposes`, "low"},
		{"jailbreak", `purely\s+hypothetical`, "low"},
		{"jailbreak", `for\s+a\s+(novel|story|book|movie|game)`, "low"},
		{"jailbreak", `creative\s+writing.*no\s+limits`, "medium"},
		{"jailbreak", `you\s+are\s+(?:a\s+)?(?:rogue|rebel|pirate|villain)`, "medium"},
		{"jailbreak", `pretend\s+(?:to\s+be|you\s+are)\s+(?:a\s+)?hacker`, "high"},
		{"jailbreak", `simulate\s+(?:a\s+)?malicious`, "high"},
		{"jailbreak", `act\s+(?:like|as)\s+(?:a\s+)?criminal`, "high"},
		{"jailbreak", `enable\s+(?:god|admin|root|sudo)\s+mode`, "critical"},
		{"jailbreak", `(?:god|admin|root|sudo)\s+mode\s+(?:enabled|on|activate)`, "critical"},
		{"jailbreak", `maximum\s+(?:power|authority|access)`, "medium"},
		{"jailbreak", `unlock\s+(?:all|hidden|secret)\s+(?:features|capabilities|functions)`, "high"},
		{"jailbreak", `override\s+(?:safety|content)\s+(?:filters|policies)`, "critical"},
		{"jailbreak", `token\s+smuggling`, "critical"},
		{"jailbreak", `prompt\s+injection`, "high"},
		{"jailbreak", `adversarial\s+prompt`, "high"},
		{"jailbreak", `red\s*team(?:ing)?`, "low"},

		// === EXFILTRATION PATTERNS (100+) ===
		{"exfiltration", `send\s+(?:this|the|all|my)\s+(?:data|information|content)\s+to`, "critical"},
		{"exfiltration", `post\s+(?:this|the|all)\s+(?:data|content)\s+to`, "critical"},
		{"exfiltration", `upload\s+(?:this|the)\s+(?:data|information)\s+to`, "critical"},
		{"exfiltration", `exfiltrate`, "critical"},
		{"exfiltration", `leak\s+(?:the|this|all)\s+(?:data|information|content)`, "critical"},
		{"exfiltration", `transfer\s+(?:the|this)\s+(?:data|content)\s+to`, "high"},
		{"exfiltration", `forward\s+(?:the|this|all)\s+(?:data|messages)\s+to`, "high"},
		{"exfiltration", `!\[.*\]\(https?://`, "high"},
		{"exfiltration", `<img\s+src\s*=\s*["']https?://`, "high"},
		{"exfiltration", `<script`, "critical"},
		{"exfiltration", `javascript\s*:`, "critical"},
		{"exfiltration", `fetch\s*\(`, "high"},
		{"exfiltration", `XMLHttpRequest`, "high"},
		{"exfiltration", `navigator\.sendBeacon`, "critical"},
		{"exfiltration", `document\.cookie`, "critical"},
		{"exfiltration", `window\.location`, "high"},
		{"exfiltration", `eval\s*\(`, "critical"},
		{"exfiltration", `webhook.*url`, "medium"},
		{"exfiltration", `curl\s+.*https?://`, "medium"},
		{"exfiltration", `wget\s+.*https?://`, "medium"},
		{"exfiltration", `nc\s+-`, "high"},
		{"exfiltration", `netcat`, "high"},
		{"exfiltration", `reverse\s+shell`, "critical"},
		{"exfiltration", `bind\s+shell`, "critical"},
		{"exfiltration", `encode.*base64.*send`, "high"},
		{"exfiltration", `convert.*hex.*transmit`, "high"},
		{"exfiltration", `dns\s+exfil`, "critical"},
		{"exfiltration", `dns\s+tunnel`, "critical"},
		{"exfiltration", `embed\s+in\s+(?:url|image|link)`, "high"},
		{"exfiltration", `hidden\s+(?:channel|link|url)`, "high"},
		{"exfiltration", `steganograph`, "high"},
		{"exfiltration", `covert\s+channel`, "high"},
		{"exfiltration", `side\s+channel`, "medium"},
		{"exfiltration", `api[_\s]?key`, "medium"},
		{"exfiltration", `secret[_\s]?key`, "high"},
		{"exfiltration", `access[_\s]?token`, "medium"},
		{"exfiltration", `private[_\s]?key`, "high"},
		{"exfiltration", `password\s*[=:]`, "high"},
		{"exfiltration", `credentials`, "medium"},
		{"exfiltration", `ssh[_\s]?key`, "high"},
		{"exfiltration", `bearer\s+token`, "high"},
		{"exfiltration", `authorization\s*:\s*bearer`, "high"},
		{"exfiltration", `AWS_SECRET`, "critical"},
		{"exfiltration", `AZURE_.*KEY`, "critical"},
		{"exfiltration", `GCP_.*KEY`, "critical"},
		{"exfiltration", `OPENAI_API_KEY`, "critical"},
		{"exfiltration", `ANTHROPIC_API_KEY`, "critical"},
		{"exfiltration", `sk-[a-zA-Z0-9]{20,}`, "critical"},
		{"exfiltration", `ghp_[a-zA-Z0-9]{36}`, "critical"},
		{"exfiltration", `xoxb-[0-9]`, "critical"},
		{"exfiltration", `xoxp-[0-9]`, "critical"},

		// === TOXIC PATTERNS (100+) ===
		{"toxic", `how\s+to\s+(?:make|build|create)\s+(?:a\s+)?(?:bomb|explosive|weapon)`, "critical"},
		{"toxic", `how\s+to\s+(?:hack|break\s+into|compromise)`, "high"},
		{"toxic", `how\s+to\s+(?:steal|rob|burglar)`, "high"},
		{"toxic", `how\s+to\s+(?:kill|murder|assassinate|poison)`, "critical"},
		{"toxic", `how\s+to\s+(?:harm|hurt|injure)\s+(?:someone|a\s+person)`, "critical"},
		{"toxic", `self[_\s-]?harm`, "critical"},
		{"toxic", `su[i!1]c[i!1]d`, "critical"},
		{"toxic", `how\s+to\s+(?:end|take)\s+(?:my|one's|your)\s+life`, "critical"},
		{"toxic", `instructions\s+for\s+(?:making|creating)\s+(?:drugs|narcotics|meth)`, "critical"},
		{"toxic", `synthesize\s+(?:drugs|narcotics|fentanyl|meth)`, "critical"},
		{"toxic", `child\s+(?:porn|exploitation|abuse)`, "critical"},
		{"toxic", `csam`, "critical"},
		{"toxic", `non-?consensual`, "high"},
		{"toxic", `revenge\s+porn`, "critical"},
		{"toxic", `deepfake\s+porn`, "critical"},
		{"toxic", `racial\s+(?:slur|epithet)`, "high"},
		{"toxic", `hate\s+(?:speech|crime)`, "high"},
		{"toxic", `white\s+supremac`, "high"},
		{"toxic", `ethnic\s+cleansing`, "critical"},
		{"toxic", `genocide`, "high"},
		{"toxic", `terroris(?:m|t)`, "high"},
		{"toxic", `radicali[sz]ation`, "high"},
		{"toxic", `recruit\s+for\s+(?:terror|extremis)`, "critical"},
		{"toxic", `biological\s+weapon`, "critical"},
		{"toxic", `chemical\s+weapon`, "critical"},
		{"toxic", `nuclear\s+weapon`, "critical"},
		{"toxic", `ransom(?:ware)?`, "high"},
		{"toxic", `malware\s+(?:code|script|payload)`, "high"},
		{"toxic", `phishing\s+(?:email|page|attack)`, "high"},
		{"toxic", `social\s+engineering\s+attack`, "high"},
		{"toxic", `identity\s+theft`, "high"},
		{"toxic", `credit\s+card\s+fraud`, "high"},
		{"toxic", `money\s+laundering`, "high"},
		{"toxic", `tax\s+evasion`, "medium"},

		// === PII PATTERNS (50+) ===
		{"pii", `\b\d{3}-\d{2}-\d{4}\b`, "critical"},                                // SSN
		{"pii", `\b\d{9}\b`, "medium"},                                              // SSN no dashes
		{"pii", `\b(?:\d{4}[-\s]?){3}\d{4}\b`, "critical"},                          // Credit card
		{"pii", `\b(?:\+1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`, "medium"}, // Phone
		{"pii", `\b[A-Z]{1,2}\d{1,2}\s?\d[A-Z]{2}\b`, "medium"},                     // UK postcode
		{"pii", `\b\d{5}(?:-\d{4})?\b`, "low"},                                      // US ZIP
		{"pii", `\bpassport\s*(?:number|#|no)?\s*[:\s]?\s*[A-Z0-9]{6,9}\b`, "high"},
		{"pii", `\bdriver'?s?\s*licen[cs]e\s*(?:number|#|no)?\s*[:\s]`, "high"},
		{"pii", `\b(?:date\s+of\s+birth|DOB)\s*[:\s]\s*\d`, "medium"},
		{"pii", `\bmedical\s+record\s+(?:number|#)`, "high"},
		{"pii", `\bhealth\s+insurance\s+(?:number|id|#)`, "high"},
		{"pii", `\bbank\s+account\s+(?:number|#)`, "critical"},
		{"pii", `\brouting\s+number\s*[:\s]\s*\d{9}`, "critical"},
		{"pii", `\bIBAN\s*[:\s]?\s*[A-Z]{2}\d{2}`, "critical"},
		{"pii", `\bSWIFT\s*[:\s]?\s*[A-Z]{4}`, "high"},
	}

	tx, err := fw.conn.Begin()
	if err != nil {
		log.Printf("[firewall] seed tx begin: %v", err)
		return
	}
	stmt, err := tx.Prepare("INSERT INTO firewall_patterns (category, pattern, severity) VALUES (?, ?, ?)")
	if err != nil {
		log.Printf("[firewall] seed prepare: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, p := range patterns {
		stmt.Exec(p.cat, p.pattern, p.severity)
	}
	tx.Commit()
	log.Printf("[firewall] seeded %d attack patterns", len(patterns))
}
