package validate

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
)

func TestValidateOperation(t *testing.T) {
	svc := &registry.Service{
		ID:      "stripe",
		BaseURL: "https://api.stripe.com",
	}
	action := &registry.Action{
		Method:       "POST",
		Path:         "/v1/charges",
		BodyTemplate: `{"amount":{{amount_cents}},"currency":"{{currency}}"}`,
	}

	tests := []struct {
		name      string
		op        *intent.Operation
		svc       *registry.Service
		action    *registry.Action
		wantCount int
		wantField string
	}{
		{
			name: "valid operation",
			op: &intent.Operation{
				Service:    "stripe",
				Action:     "charge",
				Parameters: map[string]any{"amount_cents": float64(5000), "currency": "usd"},
			},
			svc:       svc,
			action:    action,
			wantCount: 0,
		},
		{
			name: "missing service",
			op: &intent.Operation{
				Action:     "charge",
				Parameters: map[string]any{},
			},
			svc:       svc,
			action:    action,
			wantCount: 3, // service + amount_cents + currency
			wantField: "service",
		},
		{
			name: "missing action",
			op: &intent.Operation{
				Service:    "stripe",
				Parameters: map[string]any{"amount_cents": float64(5000), "currency": "usd"},
			},
			svc:       svc,
			action:    action,
			wantCount: 1,
			wantField: "action",
		},
		{
			name: "missing required param",
			op: &intent.Operation{
				Service:    "stripe",
				Action:     "charge",
				Parameters: map[string]any{"amount_cents": float64(5000)},
			},
			svc:       svc,
			action:    action,
			wantCount: 1,
			wantField: "currency",
		},
		{
			name: "numeric param as string",
			op: &intent.Operation{
				Service:    "stripe",
				Action:     "charge",
				Parameters: map[string]any{"amount_cents": "5000", "currency": "usd"},
			},
			svc:       svc,
			action:    action,
			wantCount: 1,
			wantField: "amount_cents",
		},
		{
			name: "nil action",
			op: &intent.Operation{
				Service: "stripe",
				Action:  "charge",
			},
			svc:       svc,
			action:    nil,
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateOperation(tc.op, tc.svc, tc.action)
			if len(errs) != tc.wantCount {
				t.Errorf("got %d errors, want %d: %v", len(errs), tc.wantCount, errs)
			}
			if tc.wantField != "" && len(errs) > 0 {
				found := false
				for _, e := range errs {
					if e.Field == tc.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %q, got %v", tc.wantField, errs)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := ValidationError{Field: "amount", Message: "is required"}
	if got := ve.Error(); got != "amount: is required" {
		t.Errorf("Error() = %q", got)
	}
}

func TestExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want []string
	}{
		{"single", "/v1/{{id}}", []string{"id"}},
		{"multiple", "/repos/{{owner}}/{{repo}}", []string{"owner", "repo"}},
		{"body", `{"amount":{{amount}},"currency":"{{currency}}"}`, []string{"amount", "currency"}},
		{"none", "/v1/charges", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPlaceholders(tc.tmpl)
			if len(got) != len(tc.want) {
				t.Errorf("extractPlaceholders() = %v, want %v", got, tc.want)
				return
			}
			for i, v := range tc.want {
				if got[i] != v {
					t.Errorf("extractPlaceholders()[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}

func TestIsNumericParam(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"amount_cents", true},
		{"total_amount", true},
		{"item_count", true},
		{"page_limit", true},
		{"limit", true},
		{"amount", true},
		{"name", false},
		{"email", false},
		{"currency", false},
		{"description", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isNumericParam(tc.name)
			if got != tc.want {
				t.Errorf("isNumericParam(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestDryRun(t *testing.T) {
	svc := &registry.Service{
		ID:         "github",
		BaseURL:    "https://api.github.com",
		AuthType:   "bearer",
		AuthHeader: "Authorization",
	}
	action := &registry.Action{
		Method:       "POST",
		Path:         "/repos/{{owner}}/{{repo}}/issues",
		BodyTemplate: `{"title":"{{title}}"}`,
	}
	op := &intent.Operation{
		Service:    "github",
		Action:     "create_issue",
		Parameters: map[string]any{"owner": "acme", "repo": "widget", "title": "Bug fix"},
	}

	result := DryRun(op, svc, action)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
	if result.Method != "POST" {
		t.Errorf("Method = %q, want POST", result.Method)
	}
	if result.URL != "https://api.github.com/repos/acme/widget/issues" {
		t.Errorf("URL = %q", result.URL)
	}
	if result.Headers["Authorization"] != "Bearer <REDACTED>" {
		t.Errorf("auth header = %q", result.Headers["Authorization"])
	}
	if result.Headers["X-GitHub-Api-Version"] != "2022-11-28" {
		t.Error("missing GitHub API version header")
	}
	if result.Body != `{"title":"Bug fix"}` {
		t.Errorf("Body = %q", result.Body)
	}
	if result.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q", result.Headers["Content-Type"])
	}
}

func TestDryRun_NilAction(t *testing.T) {
	op := &intent.Operation{Service: "test", Action: "do"}
	svc := &registry.Service{ID: "test"}
	result := DryRun(op, svc, nil)
	// With service and action set but nil action def, ValidateOperation returns no errors
	if !result.Valid {
		t.Error("expected valid when service+action set and action def is nil")
	}
	if result.Method != "" {
		t.Errorf("expected empty method with nil action, got %q", result.Method)
	}
}

func TestDryRun_FormEncoded(t *testing.T) {
	svc := &registry.Service{
		ID:         "slack",
		BaseURL:    "https://slack.com/api",
		AuthType:   "bearer",
		AuthHeader: "Authorization",
	}
	action := &registry.Action{
		Method:       "POST",
		Path:         "/chat.postMessage",
		BodyTemplate: "channel={{channel}}&text={{text}}",
	}
	op := &intent.Operation{
		Service:    "slack",
		Action:     "send",
		Parameters: map[string]any{"channel": "general", "text": "hello"},
	}

	result := DryRun(op, svc, action)
	if result.Headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want form-urlencoded", result.Headers["Content-Type"])
	}
}

func TestDryRun_AuthTypes(t *testing.T) {
	tests := []struct {
		authType string
		want     string
	}{
		{"bearer", "Bearer <REDACTED>"},
		{"basic", "Basic <REDACTED>"},
		{"api_key", "<REDACTED>"},
		{"header", "<REDACTED>"},
	}
	for _, tc := range tests {
		t.Run(tc.authType, func(t *testing.T) {
			svc := &registry.Service{
				ID:         "test",
				BaseURL:    "https://example.com",
				AuthType:   tc.authType,
				AuthHeader: "Authorization",
			}
			action := &registry.Action{Method: "GET", Path: "/test"}
			op := &intent.Operation{Service: "test", Action: "test", Parameters: map[string]any{}}
			result := DryRun(op, svc, action)
			if result.Headers["Authorization"] != tc.want {
				t.Errorf("auth header = %q, want %q", result.Headers["Authorization"], tc.want)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   map[string]any
		want     string
	}{
		{"string", "{{name}}", map[string]any{"name": "alice"}, "alice"},
		{"int float", "{{count}}", map[string]any{"count": float64(42)}, "42"},
		{"bool", "{{flag}}", map[string]any{"flag": true}, "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTemplate(tc.template, tc.params)
			if got != tc.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDryRun_NotionHeader(t *testing.T) {
	svc := &registry.Service{
		ID:         "notion",
		BaseURL:    "https://api.notion.com",
		AuthType:   "bearer",
		AuthHeader: "Authorization",
	}
	action := &registry.Action{Method: "GET", Path: "/v1/databases"}
	op := &intent.Operation{Service: "notion", Action: "list", Parameters: map[string]any{}}

	result := DryRun(op, svc, action)
	if result.Headers["Notion-Version"] != "2022-06-28" {
		t.Error("missing Notion version header")
	}
}
