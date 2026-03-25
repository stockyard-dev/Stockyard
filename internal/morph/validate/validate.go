// Package validate provides request validation and dry-run previews for Morph operations.
package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
)

// ValidationError describes a single validation problem.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// DryRunResult shows what would be sent without executing.
type DryRunResult struct {
	Valid      bool              `json:"valid"`
	Errors     []ValidationError `json:"errors,omitempty"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body,omitempty"`
	Service    string            `json:"service"`
	Action     string            `json:"action"`
	Parameters map[string]any    `json:"parameters"`
}

// placeholderRe matches {{param}} placeholders in templates.
var placeholderRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ValidateOperation checks that an operation has all required parameters and
// that their types are reasonable for the target action.
func ValidateOperation(op *intent.Operation, svc *registry.Service, action *registry.Action) []ValidationError {
	var errs []ValidationError

	if op.Service == "" {
		errs = append(errs, ValidationError{Field: "service", Message: "service is required"})
	}
	if op.Action == "" {
		errs = append(errs, ValidationError{Field: "action", Message: "action is required"})
	}

	if action == nil {
		return errs
	}

	// Extract required parameters from path and body templates.
	required := extractPlaceholders(action.Path)
	if action.BodyTemplate != "" {
		required = append(required, extractPlaceholders(action.BodyTemplate)...)
	}

	// De-duplicate
	seen := map[string]bool{}
	var unique []string
	for _, p := range required {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	for _, param := range unique {
		val, ok := op.Parameters[param]
		if !ok || val == nil {
			errs = append(errs, ValidationError{
				Field:   param,
				Message: fmt.Sprintf("required parameter %q is missing", param),
			})
			continue
		}
		// Basic type check: numbers-looking params should be numeric
		if isNumericParam(param) {
			switch val.(type) {
			case float64, int, int64:
				// ok
			case string:
				errs = append(errs, ValidationError{
					Field:   param,
					Message: fmt.Sprintf("parameter %q should be a number, got string", param),
				})
			}
		}
	}

	return errs
}

// DryRun builds a preview of what would be sent for the given operation without
// making any HTTP call.
func DryRun(op *intent.Operation, svc *registry.Service, action *registry.Action) *DryRunResult {
	result := &DryRunResult{
		Service:    op.Service,
		Action:     op.Action,
		Parameters: op.Parameters,
		Headers:    map[string]string{},
	}

	// Validate first.
	errs := ValidateOperation(op, svc, action)
	result.Errors = errs
	result.Valid = len(errs) == 0

	if action == nil {
		return result
	}

	// Build URL
	result.Method = action.Method
	result.URL = svc.BaseURL + renderTemplate(action.Path, op.Parameters)

	// Build auth header
	switch svc.AuthType {
	case "bearer":
		result.Headers[svc.AuthHeader] = "Bearer <REDACTED>"
	case "basic":
		result.Headers[svc.AuthHeader] = "Basic <REDACTED>"
	case "api_key", "header":
		result.Headers[svc.AuthHeader] = "<REDACTED>"
	}

	// Content type and body
	if action.BodyTemplate != "" && (action.Method == "POST" || action.Method == "PUT" || action.Method == "PATCH") {
		rendered := renderTemplate(action.BodyTemplate, op.Parameters)
		result.Body = rendered

		if strings.Contains(action.BodyTemplate, "=") {
			result.Headers["Content-Type"] = "application/x-www-form-urlencoded"
		} else {
			result.Headers["Content-Type"] = "application/json"
		}
	}

	result.Headers["Accept"] = "application/json"

	// Extra service headers
	for k, v := range action.Headers {
		result.Headers[k] = v
	}
	if svc.ID == "github" {
		result.Headers["X-GitHub-Api-Version"] = "2022-11-28"
	}
	if svc.ID == "notion" {
		result.Headers["Notion-Version"] = "2022-06-28"
	}

	return result
}

// extractPlaceholders returns all {{param}} names from a template.
func extractPlaceholders(tmpl string) []string {
	matches := placeholderRe.FindAllStringSubmatch(tmpl, -1)
	var out []string
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// isNumericParam returns true if the parameter name suggests a numeric value.
func isNumericParam(name string) bool {
	numericSuffixes := []string{"_cents", "_amount", "_count", "_limit", "_number", "_id_num"}
	nameLower := strings.ToLower(name)
	for _, suffix := range numericSuffixes {
		if strings.HasSuffix(nameLower, suffix) {
			return true
		}
	}
	return nameLower == "amount" || nameLower == "limit" || nameLower == "amount_cents"
}

// renderTemplate replaces {{param}} placeholders with parameter values.
func renderTemplate(template string, params map[string]any) string {
	result := template
	for key, val := range params {
		placeholder := "{{" + key + "}}"
		var replacement string
		switch v := val.(type) {
		case string:
			replacement = v
		case float64:
			if v == float64(int(v)) {
				replacement = fmt.Sprintf("%d", int(v))
			} else {
				replacement = fmt.Sprintf("%g", v)
			}
		case bool:
			replacement = fmt.Sprintf("%t", v)
		default:
			data, _ := json.Marshal(v)
			replacement = string(data)
		}
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	return result
}
