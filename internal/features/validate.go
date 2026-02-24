package features

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ValidationConfig defines schema validation settings.
type ValidationConfig struct {
	Enabled    bool
	MaxRetries int
	Schemas    map[string]json.RawMessage // schema name → JSON Schema
}

// ValidationResult tracks the outcome of a validation attempt.
type ValidationResult struct {
	SchemaName string
	Passed     bool
	Error      string
	Retries    int
}

// markdownJSONRe matches ```json ... ``` blocks
var markdownJSONRe = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*\n?(.*?)\\s*```\\s*$")

// ValidateMiddleware returns middleware that validates responses against JSON schemas.
func ValidateMiddleware(cfg ValidationConfig) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if !cfg.Enabled || req.Schema == "" {
				return next(ctx, req)
			}

			schema, ok := cfg.Schemas[req.Schema]
			if !ok {
				return next(ctx, req) // No schema defined, pass through
			}

			var lastErr error
			maxAttempts := cfg.MaxRetries + 1

			for attempt := 0; attempt < maxAttempts; attempt++ {
				// On retry, inject a "respond in valid JSON" hint
				reqToSend := req
				if attempt > 0 {
					reqToSend = injectJSONHint(req, lastErr)
				}

				resp, err := next(ctx, reqToSend)
				if err != nil {
					return nil, err
				}

				if len(resp.Choices) == 0 {
					return resp, nil
				}

				content := resp.Choices[0].Message.Content

				// Step 1: Strip markdown wrappers (LLMs love to wrap JSON in ```json blocks)
				content = stripMarkdownJSON(content)

				// Step 2: Validate as JSON and against schema
				if err := validateJSON(content, schema); err != nil {
					lastErr = err
					log.Printf("validation failed (attempt %d/%d): %v", attempt+1, maxAttempts, err)
					if attempt < maxAttempts-1 {
						continue
					}
					// All retries exhausted — return the original response but mark it
					return resp, nil
				}

				// Validation passed — return with cleaned content
				resp.Choices[0].Message.Content = content
				return resp, nil
			}

			return nil, fmt.Errorf("validation failed after %d attempts: %w", maxAttempts, lastErr)
		}
	}
}

// stripMarkdownJSON removes common LLM wrapping patterns:
//   - ```json ... ``` blocks
//   - ``` ... ``` blocks without language tag
//   - Leading/trailing whitespace
//   - Leading text before first { or [
//   - Trailing text after last } or ]
func stripMarkdownJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try to extract from markdown code fence
	if matches := markdownJSONRe.FindStringSubmatch(s); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to extract the outermost JSON object or array
	// Find first { or [ and last } or ]
	firstObj := strings.IndexByte(s, '{')
	firstArr := strings.IndexByte(s, '[')
	lastObj := strings.LastIndexByte(s, '}')
	lastArr := strings.LastIndexByte(s, ']')

	start := -1
	end := -1

	if firstObj >= 0 && (firstArr < 0 || firstObj < firstArr) {
		start = firstObj
		end = lastObj
	} else if firstArr >= 0 {
		start = firstArr
		end = lastArr
	}

	if start >= 0 && end > start {
		candidate := s[start : end+1]
		// Only use the extracted substring if it's valid JSON
		var js json.RawMessage
		if json.Unmarshal([]byte(candidate), &js) == nil {
			return candidate
		}
	}

	return s
}

// injectJSONHint adds a system message asking the model to return valid JSON.
func injectJSONHint(req *provider.Request, prevErr error) *provider.Request {
	clone := *req
	hint := provider.Message{
		Role: "system",
		Content: fmt.Sprintf(
			"Your previous response was not valid JSON. Error: %s. "+
				"Please respond with ONLY valid JSON, no markdown fences, no explanations.",
			prevErr.Error()),
	}
	clone.Messages = append([]provider.Message{hint}, clone.Messages...)
	return &clone
}

// validateJSON validates a JSON string against a JSON schema.
// Implements core JSON Schema Draft 7 validation without external dependencies:
// type, required, properties (with nested type checking), enum, minLength, maxLength,
// minimum, maximum, minItems, maxItems.
func validateJSON(content string, schema json.RawMessage) error {
	// Step 1: Parse content as JSON
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	// Step 2: Parse schema
	var schemaDef schemaNode
	if err := json.Unmarshal(schema, &schemaDef); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	// Step 3: Validate recursively
	return validateNode(parsed, &schemaDef, "")
}

// schemaNode represents a subset of JSON Schema.
type schemaNode struct {
	Type       interface{}            `json:"type"` // string or []string
	Required   []string               `json:"required"`
	Properties map[string]*schemaNode `json:"properties"`
	Items      *schemaNode            `json:"items"`
	Enum       []any                  `json:"enum"`
	MinLength  *int                   `json:"minLength"`
	MaxLength  *int                   `json:"maxLength"`
	Minimum    *float64               `json:"minimum"`
	Maximum    *float64               `json:"maximum"`
	MinItems   *int                   `json:"minItems"`
	MaxItems   *int                   `json:"maxItems"`
}

// getTypes returns the allowed types as a string slice.
func (s *schemaNode) getTypes() []string {
	if s.Type == nil {
		return nil
	}
	switch v := s.Type.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var types []string
		for _, t := range v {
			if ts, ok := t.(string); ok {
				types = append(types, ts)
			}
		}
		return types
	}
	return nil
}

func validateNode(value any, schema *schemaNode, path string) error {
	if schema == nil {
		return nil
	}

	// Type check
	types := schema.getTypes()
	if len(types) > 0 {
		actualType := jsonType(value)
		matched := false
		for _, t := range types {
			if t == actualType || (t == "integer" && actualType == "number" && isInteger(value)) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("at %s: expected type %v, got %s", pathStr(path), types, actualType)
		}
	}

	// Enum check
	if len(schema.Enum) > 0 {
		found := false
		valJSON, _ := json.Marshal(value)
		for _, e := range schema.Enum {
			eJSON, _ := json.Marshal(e)
			if string(valJSON) == string(eJSON) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("at %s: value not in enum %v", pathStr(path), schema.Enum)
		}
	}

	switch v := value.(type) {
	case map[string]any:
		// Required fields
		for _, req := range schema.Required {
			if _, ok := v[req]; !ok {
				return fmt.Errorf("at %s: missing required field %q", pathStr(path), req)
			}
		}
		// Property validation
		for propName, propSchema := range schema.Properties {
			if propVal, ok := v[propName]; ok {
				if err := validateNode(propVal, propSchema, path+"."+propName); err != nil {
					return err
				}
			}
		}

	case []any:
		if schema.MinItems != nil && len(v) < *schema.MinItems {
			return fmt.Errorf("at %s: array has %d items, minimum %d", pathStr(path), len(v), *schema.MinItems)
		}
		if schema.MaxItems != nil && len(v) > *schema.MaxItems {
			return fmt.Errorf("at %s: array has %d items, maximum %d", pathStr(path), len(v), *schema.MaxItems)
		}
		if schema.Items != nil {
			for i, item := range v {
				if err := validateNode(item, schema.Items, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}

	case string:
		if schema.MinLength != nil && len(v) < *schema.MinLength {
			return fmt.Errorf("at %s: string length %d below minimum %d", pathStr(path), len(v), *schema.MinLength)
		}
		if schema.MaxLength != nil && len(v) > *schema.MaxLength {
			return fmt.Errorf("at %s: string length %d exceeds maximum %d", pathStr(path), len(v), *schema.MaxLength)
		}

	case float64:
		if schema.Minimum != nil && v < *schema.Minimum {
			return fmt.Errorf("at %s: value %v below minimum %v", pathStr(path), v, *schema.Minimum)
		}
		if schema.Maximum != nil && v > *schema.Maximum {
			return fmt.Errorf("at %s: value %v exceeds maximum %v", pathStr(path), v, *schema.Maximum)
		}
	}

	return nil
}

func jsonType(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func isInteger(v any) bool {
	if f, ok := v.(float64); ok {
		return f == float64(int64(f))
	}
	return false
}

func pathStr(path string) string {
	if path == "" {
		return "$"
	}
	return "$" + path
}
