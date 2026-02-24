package tracker

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestCountInputTokens(t *testing.T) {
	messages := []provider.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello, how are you today?"},
	}

	tokens := CountInputTokens("gpt-4o-mini", messages)
	// "system: You are helpful.\nuser: Hello, how are you today?\n" = ~56 chars / 4 = ~14 tokens
	if tokens < 5 || tokens > 50 {
		t.Errorf("CountInputTokens = %d, expected between 5 and 50", tokens)
	}
}

func TestCountOutputTokens(t *testing.T) {
	tests := []struct {
		content string
		wantMin int
		wantMax int
	}{
		{"Hello!", 1, 3},
		{"This is a longer response that should have more tokens.", 5, 25},
		{"", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.content[:min(20, len(tt.content))], func(t *testing.T) {
			got := CountOutputTokens(tt.content)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CountOutputTokens(%q) = %d, want [%d, %d]", tt.content, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCountChunkTokens(t *testing.T) {
	// Empty chunk = 0
	if got := CountChunkTokens(""); got != 0 {
		t.Errorf("CountChunkTokens('') = %d, want 0", got)
	}
	// Short chunk = at least 1
	if got := CountChunkTokens("Hi"); got < 1 {
		t.Errorf("CountChunkTokens('Hi') = %d, want >= 1", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
