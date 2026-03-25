// Package detect provides feedback loop and emergence detection.
package detect

// LoopCandidate represents a potential feedback loop.
type LoopCandidate struct {
	Components []string `json:"components"`
	Type       string   `json:"type"`
	Strength   float64  `json:"strength"`
}

// FindLoops searches for feedback loops in a component chain.
func FindLoops(chains [][]string) []LoopCandidate {
	var loops []LoopCandidate
	for _, chain := range chains {
		if len(chain) < 2 { continue }
		seen := map[string]bool{}
		for _, c := range chain {
			if seen[c] {
				loops = append(loops, LoopCandidate{Components: chain, Type: "positive_feedback", Strength: 0.5})
				break
			}
			seen[c] = true
		}
	}
	return loops
}
