package aigen

import (
	"context"
	"sync"
	"testing"
)

// TestSelfTestEvalCatchesInconsistency is the regression test for the slop
// case I found by reading production logs on 2026-04-08: the model returned
//
//	{"duration_min": 50, "name": "45-min massage"}
//
// Both fields are schema-valid (name is a string under 60 chars, duration is
// an integer in 5-240 range). Both old evals passed. But the content is
// internally inconsistent: the name references 45 minutes while the field
// says 50. A customer reading that in their dashboard would immediately
// distrust the AI suggestion. The cross-field eval added in builtins.go
// catches this case by extracting any number from the name and comparing
// to duration_min.
func TestSelfTestEvalCatchesInconsistency(t *testing.T) {
	// Fresh registry for this test
	registryMu.Lock()
	registry = map[string]Task{}
	registryMu.Unlock()
	// Reset the sync.Once so RegisterBuiltins runs again
	builtinsRegistered = sync.Once{}
	RegisterBuiltins()

	cases := []struct {
		name      string
		modelOut  string
		wantError string
	}{
		{
			name:      "exact slop case from production",
			modelOut:  `{"duration_min": 50, "name": "45-min massage"}`,
			wantError: "name has 45 but duration_min=50",
		},
		{
			name:      "numbers match in minutes",
			modelOut:  `{"duration_min": 60, "name": "60-min deep tissue massage"}`,
			wantError: "",
		},
		{
			name:      "hour reference matches",
			modelOut:  `{"duration_min": 60, "name": "1-hour haircut"}`,
			wantError: "",
		},
		{
			name:      "no number in name — should pass",
			modelOut:  `{"duration_min": 45, "name": "Initial intake"}`,
			wantError: "",
		},
		{
			name:      "name says 90 but field says 30",
			modelOut:  `{"duration_min": 30, "name": "90-minute session"}`,
			wantError: "name has 90 but duration_min=30",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
				return tc.modelOut, nil
			})
			_, err := Generate(context.Background(), Request{
				Task: "aigen.self_test",
			})
			if tc.wantError == "" {
				if err != nil {
					t.Errorf("expected pass, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantError)
				}
				if !contains(err.Error(), tc.wantError) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantError)
				}
			}
		})
	}
}
