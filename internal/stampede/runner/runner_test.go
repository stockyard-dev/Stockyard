package runner

import (
	"math"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
)

func TestEqualDistribution(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		wantEach float64
	}{
		{"single persona", 1, 1.0},
		{"two personas", 2, 0.5},
		{"three personas", 3, 1.0 / 3.0},
		{"five personas", 5, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			personas := make([]persona.Persona, tt.count)
			for i := range personas {
				personas[i] = persona.Persona{Name: string(rune('A' + i))}
			}

			dist := EqualDistribution(personas)
			if len(dist) != tt.count {
				t.Fatalf("expected %d weights, got %d", tt.count, len(dist))
			}

			sum := 0.0
			for _, w := range dist {
				if math.Abs(w-tt.wantEach) > 1e-9 {
					t.Errorf("expected weight %f, got %f", tt.wantEach, w)
				}
				sum += w
			}
			if math.Abs(sum-1.0) > 1e-9 {
				t.Errorf("expected sum 1.0, got %f", sum)
			}
		})
	}
}

func TestSelectPersona_RespectsDistribution(t *testing.T) {
	personas := []persona.Persona{
		{Name: "confused"},
		{Name: "power"},
		{Name: "adversarial"},
	}
	// 90% confused, 5% power, 5% adversarial
	dist := []float64{0.9, 0.05, 0.05}

	counts := map[string]int{}
	iterations := 10000
	for i := 0; i < iterations; i++ {
		p := selectPersona(personas, dist)
		counts[p.Name]++
	}

	// Confused should dominate
	confusedPct := float64(counts["confused"]) / float64(iterations)
	if confusedPct < 0.8 {
		t.Errorf("expected confused >= 80%%, got %.1f%%", confusedPct*100)
	}

	// Each persona should appear at least once with this many iterations
	for _, p := range personas {
		if counts[p.Name] == 0 {
			t.Errorf("persona %q was never selected in %d iterations", p.Name, iterations)
		}
	}
}

func TestSelectPersona_FallbackToLast(t *testing.T) {
	personas := []persona.Persona{
		{Name: "first"},
		{Name: "last"},
	}
	// Distribution sums to less than 1.0, so when rand > cumulative,
	// fallback returns last persona
	dist := []float64{0.0, 0.0}

	p := selectPersona(personas, dist)
	if p.Name != "last" {
		t.Errorf("expected fallback to last persona, got %q", p.Name)
	}
}

func TestParseDistribution(t *testing.T) {
	personas := []persona.Persona{
		{Name: "confused"},
		{Name: "power"},
		{Name: "adversarial"},
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, dist []float64)
	}{
		{
			name:  "standard distribution",
			input: "confused:60%,power:25%,adversarial:15%",
			check: func(t *testing.T, dist []float64) {
				if math.Abs(dist[0]-0.60) > 0.01 {
					t.Errorf("confused: want 0.60, got %f", dist[0])
				}
				if math.Abs(dist[1]-0.25) > 0.01 {
					t.Errorf("power: want 0.25, got %f", dist[1])
				}
				if math.Abs(dist[2]-0.15) > 0.01 {
					t.Errorf("adversarial: want 0.15, got %f", dist[2])
				}
			},
		},
		{
			name:  "partial match works",
			input: "confus:80%,power:20%",
			check: func(t *testing.T, dist []float64) {
				if dist[0] < 0.7 {
					t.Errorf("confused (partial match): want ~0.8, got %f", dist[0])
				}
			},
		},
		{
			name:  "normalizes non-100 total",
			input: "confused:50%,power:50%",
			check: func(t *testing.T, dist []float64) {
				sum := 0.0
				for _, d := range dist {
					sum += d
				}
				if math.Abs(sum-1.0) > 0.01 {
					t.Errorf("expected normalized sum ~1.0, got %f", sum)
				}
			},
		},
		{
			name:    "invalid format",
			input:   "confused60%",
			wantErr: true,
		},
		{
			name:    "unknown persona",
			input:   "nonexistent:100%",
			wantErr: true,
		},
		{
			name:    "invalid percentage",
			input:   "confused:abc%",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := ParseDistribution(tt.input, personas)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, dist)
			}
		})
	}
}
