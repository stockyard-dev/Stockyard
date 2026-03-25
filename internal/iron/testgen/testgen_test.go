package testgen

import (
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/iron/parser"
)

func makeSpec(lang string) *parser.Spec {
	return &parser.Spec{
		Name:     "TestApp",
		Language: lang,
		Features: []parser.Feature{
			{
				Name: "Users",
				Behaviors: []parser.Behavior{
					{
						Name: "Create User",
						Acceptance: []parser.Acceptance{
							{
								Description: "creates a new user",
								Method:      "POST",
								Path:        "/api/users",
								Body:        map[string]any{"name": "Alice", "email": "alice@example.com"},
								Headers:     map[string]string{"Authorization": "Bearer token"},
								Expect: parser.Expectation{
									Status:      201,
									Contains:    []string{"Alice"},
									NotContains: []string{"error"},
									Headers:     map[string]string{"Content-Type": "application/json"},
									Body:        map[string]any{"name": "Alice"},
								},
							},
						},
					},
					{
						Name: "List Users",
						Acceptance: []parser.Acceptance{
							{
								Description: "returns empty list",
								Method:      "GET",
								Path:        "/api/users",
								Expect: parser.Expectation{
									Status: 200,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestGenerateTests_Go(t *testing.T) {
	spec := makeSpec("go")
	tests, code := GenerateTests(spec)

	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}

	if !strings.Contains(code, "package main_test") {
		t.Error("expected Go test package declaration")
	}
	if !strings.Contains(code, "func Test_") {
		t.Error("expected Go test function")
	}
	if !strings.Contains(code, "POST") {
		t.Error("expected POST method in test")
	}
	if !strings.Contains(code, "/api/users") {
		t.Error("expected path in test")
	}
	if !strings.Contains(code, "201") {
		t.Error("expected status 201 in test")
	}
	if !strings.Contains(code, "Authorization") {
		t.Error("expected Authorization header in test")
	}
}

func TestGenerateTests_Python(t *testing.T) {
	spec := makeSpec("python")
	tests, code := GenerateTests(spec)

	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
	if !strings.Contains(code, "import pytest") {
		t.Error("expected pytest import")
	}
	if !strings.Contains(code, "def test_") {
		t.Error("expected Python test function")
	}
}

func TestGenerateTests_TypeScript(t *testing.T) {
	spec := makeSpec("typescript")
	tests, code := GenerateTests(spec)

	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
	if !strings.Contains(code, "describe(") {
		t.Error("expected describe block")
	}
	if !strings.Contains(code, "it(") {
		t.Error("expected it block")
	}
}

func TestGenerateTests_DefaultLanguage(t *testing.T) {
	spec := makeSpec("")
	tests, code := GenerateTests(spec)

	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
	// Default should be Go
	if !strings.Contains(code, "package main_test") {
		t.Error("expected Go test as default")
	}
}

func TestGenerateTests_NoFeatures(t *testing.T) {
	spec := &parser.Spec{Language: "go"}
	tests, code := GenerateTests(spec)

	if len(tests) != 0 {
		t.Errorf("expected 0 tests, got %d", len(tests))
	}
	// Should still have header
	if !strings.Contains(code, "package main_test") {
		t.Error("expected package header even with no features")
	}
}

func TestGenerateTests_GoBody(t *testing.T) {
	spec := makeSpec("go")
	_, code := GenerateTests(spec)

	// Should contain json.Marshal for body
	if !strings.Contains(code, "json.Marshal") {
		t.Error("expected json.Marshal for POST body")
	}
	if !strings.Contains(code, "Content-Type") {
		t.Error("expected Content-Type header for POST body")
	}
}

func TestGenerateTests_GoExpectBody(t *testing.T) {
	spec := makeSpec("go")
	_, code := GenerateTests(spec)

	if !strings.Contains(code, "json.Unmarshal") {
		t.Error("expected json.Unmarshal for body assertions")
	}
}

func TestGenerateTests_GoContains(t *testing.T) {
	spec := makeSpec("go")
	_, code := GenerateTests(spec)

	if !strings.Contains(code, "strings.Contains") {
		t.Error("expected strings.Contains for contains assertions")
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "Hello_World"},
		{"foo-bar.baz", "foo_bar_baz"},
		{"no$pecial@chars!", "nopecialchars"},
		{"already_ok", "already_ok"},
		{"123test", "123test"},
	}

	for _, tt := range tests {
		got := sanitize(tt.input)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeLower(t *testing.T) {
	got := sanitize_lower("Hello World")
	if got != "hello_world" {
		t.Errorf("sanitize_lower got %q, want %q", got, "hello_world")
	}
}

func TestGoLiteral(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", `"hello"`},
		{float64(42), "float64(42)"},
		{float64(3.14), "3.14"},
		{42, "float64(42)"},
		{true, "true"},
		{false, "false"},
	}

	for _, tt := range tests {
		got := goLiteral(tt.input)
		if got != tt.want {
			t.Errorf("goLiteral(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPythonLiteral(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", `"hello"`},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "True"},
		{false, "False"},
	}

	for _, tt := range tests {
		got := pythonLiteral(tt.input)
		if got != tt.want {
			t.Errorf("pythonLiteral(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTsLiteral(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", "'hello'"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
	}

	for _, tt := range tests {
		got := tsLiteral(tt.input)
		if got != tt.want {
			t.Errorf("tsLiteral(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGeneratedTest_Fields(t *testing.T) {
	spec := makeSpec("go")
	tests, _ := GenerateTests(spec)

	test := tests[0]
	if test.Feature != "Users" {
		t.Errorf("expected feature Users, got %s", test.Feature)
	}
	if test.Behavior != "Create User" {
		t.Errorf("expected behavior Create User, got %s", test.Behavior)
	}
	if test.Description != "creates a new user" {
		t.Errorf("expected description, got %s", test.Description)
	}
	if !strings.HasPrefix(test.Name, "Test_") {
		t.Errorf("expected test name to start with Test_, got %s", test.Name)
	}
	if test.Code == "" {
		t.Error("expected non-empty code")
	}
}
