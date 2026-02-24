package provider

import "testing"

func TestGetPricing(t *testing.T) {
	tests := []struct {
		model    string
		wantOK   bool
		wantProv string
	}{
		{"gpt-4o-mini", true, "openai"},
		{"gpt-4o", true, "openai"},
		{"claude-opus-4-6", true, "anthropic"},
		{"claude-haiku-4-5-20251001", true, "anthropic"},
		{"gemini-2.0-flash", true, "gemini"},
		{"llama-3.3-70b-versatile", true, "groq"},
		{"nonexistent-model", false, ""},
		// Test prefix matching: versioned model name
		{"gpt-4o-mini-2024-07-18", true, "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p, ok := GetPricing(tt.model)
			if ok != tt.wantOK {
				t.Errorf("GetPricing(%q) ok = %v, want %v", tt.model, ok, tt.wantOK)
			}
			if ok && p.Provider != tt.wantProv {
				t.Errorf("GetPricing(%q) provider = %q, want %q", tt.model, p.Provider, tt.wantProv)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model    string
		input    int
		output   int
		wantMin  float64
		wantMax  float64
	}{
		// gpt-4o-mini: $0.15/1M input, $0.60/1M output
		{"gpt-4o-mini", 1000, 500, 0.000_45, 0.000_46},
		// 1000 * 0.15/1M = 0.000150
		// 500 * 0.60/1M = 0.000300
		// total = 0.000450

		// gpt-4o: $2.50/1M input, $10.00/1M output
		{"gpt-4o", 10000, 5000, 0.074, 0.076},

		// Unknown model: fallback pricing
		{"unknown-model", 1000, 500, 0, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.input, tt.output)
			if cost < tt.wantMin || cost > tt.wantMax {
				t.Errorf("CalculateCost(%q, %d, %d) = %f, want [%f, %f]",
					tt.model, tt.input, tt.output, cost, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestProviderForModel(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gpt-4o-mini", "openai"},
		{"gpt-4o", "openai"},
		{"o1", "openai"},
		{"o3-mini", "openai"},
		{"claude-opus-4-6", "anthropic"},
		{"claude-sonnet-4-5-20250929", "anthropic"},
		{"gemini-2.0-flash", "gemini"},
		{"gemini-1.5-pro", "gemini"},
		{"llama-3.3-70b-versatile", "groq"},
		{"mixtral-8x7b-32768", "groq"},
		{"some-random-model", "openai"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := ProviderForModel(tt.model)
			if got != tt.want {
				t.Errorf("ProviderForModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}
