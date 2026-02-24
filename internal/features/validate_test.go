package features

import (
	"encoding/json"
	"testing"
)

func TestStripMarkdownJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean json",
			input: `{"name":"Alice","age":30}`,
			want:  `{"name":"Alice","age":30}`,
		},
		{
			name:  "json with markdown fence",
			input: "```json\n{\"name\":\"Alice\"}\n```",
			want:  `{"name":"Alice"}`,
		},
		{
			name:  "json with plain fence",
			input: "```\n{\"name\":\"Alice\"}\n```",
			want:  `{"name":"Alice"}`,
		},
		{
			name:  "json with leading text",
			input: "Here is the result:\n{\"name\":\"Alice\"}",
			want:  `{"name":"Alice"}`,
		},
		{
			name:  "json with trailing text",
			input: "{\"name\":\"Alice\"}\nHope this helps!",
			want:  `{"name":"Alice"}`,
		},
		{
			name:  "json array in markdown",
			input: "```json\n[1,2,3]\n```",
			want:  `[1,2,3]`,
		},
		{
			name:  "whitespace padding",
			input: "  \n  {\"ok\": true}  \n  ",
			want:  `{"ok": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownJSON(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateJSON_ValidJSON(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["name","age"],"properties":{"name":{"type":"string"},"age":{"type":"number"}}}`)

	// Valid
	err := validateJSON(`{"name":"Alice","age":30}`, schema)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateJSON_MissingRequired(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["name","age"],"properties":{"name":{"type":"string"},"age":{"type":"number"}}}`)

	err := validateJSON(`{"name":"Alice"}`, schema)
	if err == nil {
		t.Error("expected error for missing required field 'age'")
	}
}

func TestValidateJSON_WrongType(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"age":{"type":"number"}}}`)

	err := validateJSON(`{"age":"not a number"}`, schema)
	if err == nil {
		t.Error("expected error for wrong type on 'age'")
	}
}

func TestValidateJSON_InvalidJSON(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)

	err := validateJSON(`this is not json`, schema)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateJSON_TopLevelTypeCheck(t *testing.T) {
	schema := json.RawMessage(`{"type":"array"}`)

	err := validateJSON(`{"key":"value"}`, schema)
	if err == nil {
		t.Error("expected error: got object, wanted array")
	}

	err = validateJSON(`[1,2,3]`, schema)
	if err != nil {
		t.Errorf("expected valid array, got: %v", err)
	}
}

func TestValidateJSON_Enum(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["active","inactive"]}}}`)

	err := validateJSON(`{"status":"active"}`, schema)
	if err != nil {
		t.Errorf("expected valid enum, got: %v", err)
	}

	err = validateJSON(`{"status":"deleted"}`, schema)
	if err == nil {
		t.Error("expected error for value not in enum")
	}
}

func TestValidateJSON_ArrayItems(t *testing.T) {
	schema := json.RawMessage(`{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":3}`)

	err := validateJSON(`["a","b"]`, schema)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	err = validateJSON(`[]`, schema)
	if err == nil {
		t.Error("expected error for empty array (minItems: 1)")
	}

	err = validateJSON(`["a","b","c","d"]`, schema)
	if err == nil {
		t.Error("expected error for 4 items (maxItems: 3)")
	}

	err = validateJSON(`["a",123]`, schema)
	if err == nil {
		t.Error("expected error for non-string item")
	}
}

func TestValidateJSON_StringConstraints(t *testing.T) {
	schema := json.RawMessage(`{"type":"string","minLength":2,"maxLength":5}`)

	err := validateJSON(`"abc"`, schema)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	err = validateJSON(`"a"`, schema)
	if err == nil {
		t.Error("expected error for string too short")
	}

	err = validateJSON(`"toolong"`, schema)
	if err == nil {
		t.Error("expected error for string too long")
	}
}

func TestValidateJSON_NumberConstraints(t *testing.T) {
	schema := json.RawMessage(`{"type":"number","minimum":0,"maximum":100}`)

	err := validateJSON(`50`, schema)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	err = validateJSON(`-1`, schema)
	if err == nil {
		t.Error("expected error for number below minimum")
	}

	err = validateJSON(`101`, schema)
	if err == nil {
		t.Error("expected error for number above maximum")
	}
}

func TestValidateJSON_IntegerType(t *testing.T) {
	schema := json.RawMessage(`{"type":"integer"}`)

	err := validateJSON(`42`, schema)
	if err != nil {
		t.Errorf("expected valid integer, got: %v", err)
	}

	err = validateJSON(`3.14`, schema)
	if err == nil {
		t.Error("expected error for non-integer number")
	}
}

func TestValidateJSON_NestedObjects(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"required": ["user"],
		"properties": {
			"user": {
				"type": "object",
				"required": ["name", "email"],
				"properties": {
					"name": {"type": "string"},
					"email": {"type": "string"}
				}
			}
		}
	}`)

	err := validateJSON(`{"user":{"name":"Alice","email":"a@b.com"}}`, schema)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	err = validateJSON(`{"user":{"name":"Alice"}}`, schema)
	if err == nil {
		t.Error("expected error for missing nested required field 'email'")
	}
}
