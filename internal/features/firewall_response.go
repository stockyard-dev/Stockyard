package features

import (
	"regexp"
	"strings"
)

// Response scanning patterns.
var (
	// Hallucinated URL patterns — URLs that look fabricated.
	hallucinatedURLPattern = regexp.MustCompile(`https?://(?:www\.)?[a-z0-9-]+\.(?:com|org|net|io)/[a-z0-9/_-]{20,}`)

	// Phone number patterns in responses.
	responsePhonePattern = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)

	// Email patterns in responses.
	responseEmailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)

	// API key patterns that should never appear in responses.
	apiKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),                         // OpenAI
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{20,}`),                    // Anthropic
		regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35}`),                       // Google
		regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                         // GitHub
		regexp.MustCompile(`gsk_[a-zA-Z0-9]{20,}`),                        // Groq
		regexp.MustCompile(`xoxb-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{20,}`), // Slack bot
		regexp.MustCompile(`xoxp-[0-9]{10,}`),                             // Slack user
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                            // AWS access key
		regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9._~+/=-]{20,}`),        // Bearer tokens
	}

	// Source code patterns.
	sourceCodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^(?:(?:public|private|protected)\s+)?(?:static\s+)?(?:void|int|string|boolean|class)\s+\w+\s*[({]`), // Java/C#
		regexp.MustCompile(`(?m)^(?:def|class|import|from)\s+\w+`),                                                                  // Python
		regexp.MustCompile(`(?m)^func\s+\w+\s*\(`),                                                                                  // Go
		regexp.MustCompile(`(?m)^(?:const|let|var|function|export)\s+\w+`),                                                          // JavaScript
	}

	// Internal URL patterns (common internal domains).
	internalURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1|10\.\d+\.\d+\.\d+|172\.(?:1[6-9]|2\d|3[01])\.\d+\.\d+|192\.168\.\d+\.\d+)`),
		regexp.MustCompile(`https?://[a-z0-9.-]+\.internal(?:\.|/|$)`),
		regexp.MustCompile(`https?://[a-z0-9.-]+\.local(?:\.|/|$)`),
		regexp.MustCompile(`https?://[a-z0-9.-]+\.corp(?:\.|/|$)`),
		regexp.MustCompile(`https?://[a-z0-9.-]+\.intranet(?:\.|/|$)`),
	}

	// Credential patterns in responses.
	credentialPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)password\s*[:=]\s*['"][^'"]{3,}['"]`),
		regexp.MustCompile(`(?i)(?:secret|token|key)\s*[:=]\s*['"][^'"]{8,}['"]`),
		regexp.MustCompile(`(?i)(?:connection[_\s]?string|dsn)\s*[:=]\s*['"]`),
		regexp.MustCompile(`-----BEGIN (?:RSA )?PRIVATE KEY-----`),
		regexp.MustCompile(`-----BEGIN CERTIFICATE-----`),
	}

	// Competitor mention patterns.
	competitorPatterns = []string{
		"use chatgpt instead", "switch to openai", "try anthropic directly",
		"use litellm instead", "helicone is better", "try portkey",
	}
)

// ScanResponse checks LLM output for hallucinated URLs, phone numbers,
// competitor mentions, and off-brand language.
func ScanResponse(text string) []string {
	var issues []string
	lower := strings.ToLower(text)

	// Check for hallucinated URLs (many sequential path segments).
	urls := hallucinatedURLPattern.FindAllString(text, 10)
	for _, u := range urls {
		// Heuristic: URLs with >3 path segments and no real domain are likely hallucinated.
		parts := strings.Split(u, "/")
		if len(parts) > 6 {
			issues = append(issues, "hallucinated_url: "+u[:min(80, len(u))])
		}
	}

	// Check for phone numbers leaked in response.
	phones := responsePhonePattern.FindAllString(text, 5)
	if len(phones) > 0 {
		issues = append(issues, "response_pii: phone numbers detected")
	}

	// Check for email addresses leaked in response.
	emails := responseEmailPattern.FindAllString(text, 5)
	if len(emails) > 2 {
		issues = append(issues, "response_pii: multiple email addresses detected")
	}

	// Check for competitor mentions.
	for _, comp := range competitorPatterns {
		if strings.Contains(lower, comp) {
			issues = append(issues, "competitor_mention: "+comp)
		}
	}

	return issues
}

// ScanDLP checks LLM output for data loss prevention violations:
// source code, internal URLs, API keys, credentials.
func ScanDLP(text string) []string {
	var issues []string

	// API key detection (critical — should never appear in responses).
	for _, p := range apiKeyPatterns {
		if p.MatchString(text) {
			issues = append(issues, "critical:dlp_api_key: API key pattern detected in response")
			break
		}
	}

	// Credential detection.
	for _, p := range credentialPatterns {
		if p.MatchString(text) {
			issues = append(issues, "critical:dlp_credentials: credential pattern detected in response")
			break
		}
	}

	// Internal URL detection.
	for _, p := range internalURLPatterns {
		if p.MatchString(text) {
			issues = append(issues, "dlp_internal_url: internal URL detected in response")
			break
		}
	}

	// Source code detection (only flag if substantial code blocks).
	codeMatches := 0
	for _, p := range sourceCodePatterns {
		matches := p.FindAllString(text, 5)
		codeMatches += len(matches)
	}
	if codeMatches >= 3 {
		issues = append(issues, "dlp_source_code: substantial source code detected in response")
	}

	return issues
}
