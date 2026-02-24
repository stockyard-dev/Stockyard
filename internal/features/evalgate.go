package features

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Validator checks a response and returns pass/fail with a reason.
type Validator interface {
	Name() string
	Validate(content string) (bool, string)
}

// JSONParseValidator checks if the response is valid JSON.
type JSONParseValidator struct{}

func (v JSONParseValidator) Name() string { return "json_parse" }
func (v JSONParseValidator) Validate(content string) (bool, string) {
	// Try to find JSON in the response (may be wrapped in markdown fences)
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return false, "no JSON found in response"
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &js); err != nil {
		return false, fmt.Sprintf("invalid JSON: %v", err)
	}
	return true, ""
}

// MinLengthValidator checks minimum response length.
type MinLengthValidator struct {
	MinChars int
}

func (v MinLengthValidator) Name() string { return "min_length" }
func (v MinLengthValidator) Validate(content string) (bool, string) {
	if len(strings.TrimSpace(content)) < v.MinChars {
		return false, fmt.Sprintf("response too short: %d chars (min: %d)", len(content), v.MinChars)
	}
	return true, ""
}

// MaxLengthValidator checks maximum response length.
type MaxLengthValidator struct {
	MaxChars int
}

func (v MaxLengthValidator) Name() string { return "max_length" }
func (v MaxLengthValidator) Validate(content string) (bool, string) {
	if len(content) > v.MaxChars {
		return false, fmt.Sprintf("response too long: %d chars (max: %d)", len(content), v.MaxChars)
	}
	return true, ""
}

// RegexValidator checks if the response matches a regex pattern.
type RegexValidator struct {
	Pattern *regexp.Regexp
	Raw     string
}

func (v RegexValidator) Name() string { return "regex" }
func (v RegexValidator) Validate(content string) (bool, string) {
	if !v.Pattern.MatchString(content) {
		return false, fmt.Sprintf("response does not match pattern: %s", v.Raw)
	}
	return true, ""
}

// ContainsValidator checks if the response contains required text.
type ContainsValidator struct {
	Required string
}

func (v ContainsValidator) Name() string { return "contains" }
func (v ContainsValidator) Validate(content string) (bool, string) {
	if !strings.Contains(strings.ToLower(content), strings.ToLower(v.Required)) {
		return false, fmt.Sprintf("response missing required content: %q", v.Required)
	}
	return true, ""
}

// NotEmptyValidator checks that the response is not empty.
type NotEmptyValidator struct{}

func (v NotEmptyValidator) Name() string { return "not_empty" }
func (v NotEmptyValidator) Validate(content string) (bool, string) {
	if strings.TrimSpace(content) == "" {
		return false, "response is empty"
	}
	return true, ""
}

// EvalGateManager manages response validation and retry logic.
type EvalGateManager struct {
	validators  []validatorEntry
	retryBudget int
	passCount   atomic.Int64
	failCount   atomic.Int64
	retryCount  atomic.Int64
}

type validatorEntry struct {
	validator Validator
	action    string // retry, warn, log
}

// NewEvalGate creates an eval gate from config.
func NewEvalGate(cfg config.EvalGateConfig) *EvalGateManager {
	eg := &EvalGateManager{
		retryBudget: cfg.RetryBudget,
	}
	if eg.retryBudget <= 0 {
		eg.retryBudget = 2
	}

	for _, vc := range cfg.Validators {
		var v Validator
		action := vc.Action
		if action == "" {
			action = "retry"
		}

		switch vc.Name {
		case "json_parse":
			v = JSONParseValidator{}
		case "min_length":
			n, _ := strconv.Atoi(vc.Params)
			if n <= 0 {
				n = 10
			}
			v = MinLengthValidator{MinChars: n}
		case "max_length":
			n, _ := strconv.Atoi(vc.Params)
			if n <= 0 {
				n = 10000
			}
			v = MaxLengthValidator{MaxChars: n}
		case "regex":
			compiled, err := regexp.Compile(vc.Params)
			if err != nil {
				log.Printf("evalgate: invalid regex %q: %v", vc.Params, err)
				continue
			}
			v = RegexValidator{Pattern: compiled, Raw: vc.Params}
		case "contains":
			v = ContainsValidator{Required: vc.Params}
		case "not_empty":
			v = NotEmptyValidator{}
		default:
			log.Printf("evalgate: unknown validator %q", vc.Name)
			continue
		}

		eg.validators = append(eg.validators, validatorEntry{validator: v, action: action})
	}

	return eg
}

// Evaluate runs all validators on the response content.
// Returns pass/fail and a list of failure reasons.
func (eg *EvalGateManager) Evaluate(content string) (bool, []string) {
	var failures []string
	needsRetry := false

	for _, ve := range eg.validators {
		pass, reason := ve.validator.Validate(content)
		if !pass {
			failures = append(failures, fmt.Sprintf("[%s] %s", ve.validator.Name(), reason))
			if ve.action == "retry" {
				needsRetry = true
			}
		}
	}

	if len(failures) == 0 {
		eg.passCount.Add(1)
		return true, nil
	}

	eg.failCount.Add(1)
	if !needsRetry {
		// Only warnings/logs — still passes
		return true, failures
	}
	return false, failures
}

// Stats returns gate statistics.
func (eg *EvalGateManager) Stats() map[string]any {
	return map[string]any{
		"pass_count":   eg.passCount.Load(),
		"fail_count":   eg.failCount.Load(),
		"retry_count":  eg.retryCount.Load(),
		"retry_budget": eg.retryBudget,
		"validators":   len(eg.validators),
	}
}

// EvalGateMiddleware returns middleware that validates responses and retries on failure.
func EvalGateMiddleware(gate *EvalGateManager) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			var lastResp *provider.Response
			var lastFailures []string

			for attempt := 0; attempt <= gate.retryBudget; attempt++ {
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}

				// Get response content
				content := ""
				if len(resp.Choices) > 0 {
					content = resp.Choices[0].Message.Content
				}

				// Evaluate
				pass, failures := gate.Evaluate(content)

				if pass {
					if attempt > 0 {
						log.Printf("evalgate: passed on attempt %d", attempt+1)
					}
					return resp, nil
				}

				lastResp = resp
				lastFailures = failures

				if attempt < gate.retryBudget {
					gate.retryCount.Add(1)
					log.Printf("evalgate: attempt %d/%d failed: %s — retrying",
						attempt+1, gate.retryBudget+1, strings.Join(failures, "; "))
				}
			}

			// Exhausted retry budget — return last response with warning
			log.Printf("evalgate: all %d attempts failed: %s", gate.retryBudget+1,
				strings.Join(lastFailures, "; "))

			if lastResp != nil && lastResp.Choices != nil {
				// Tag the response so the caller knows it failed validation
				if req.Extra == nil {
					req.Extra = make(map[string]any)
				}
				req.Extra["_eval_failed"] = true
				req.Extra["_eval_failures"] = lastFailures
			}

			return lastResp, nil
		}
	}
}

// extractJSON tries to find a JSON object or array in the text,
// handling common LLM patterns like markdown code fences.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try direct parse first
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return s
	}

	// Try extracting from markdown code fences
	fenceStarts := []string{"```json\n", "```json\r\n", "```\n", "```\r\n"}
	for _, start := range fenceStarts {
		if idx := strings.Index(s, start); idx >= 0 {
			content := s[idx+len(start):]
			if end := strings.Index(content, "```"); end > 0 {
				return strings.TrimSpace(content[:end])
			}
		}
	}

	// Try finding first { or [ and matching bracket
	for i, ch := range s {
		if ch == '{' || ch == '[' {
			closing := byte('}')
			if ch == '[' {
				closing = ']'
			}
			depth := 0
			for j := i; j < len(s); j++ {
				if s[j] == byte(ch) {
					depth++
				} else if s[j] == closing {
					depth--
					if depth == 0 {
						return s[i : j+1]
					}
				}
			}
		}
	}

	return ""
}
