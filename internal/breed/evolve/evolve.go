// Package evolve provides genetic operators for prompt evolution.
package evolve

import (
	"math/rand"
	"strings"
)

// Mutate applies random mutations to a prompt string.
func Mutate(prompt string, rate float64) (string, []string) {
	var mutations []string
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return prompt, nil
	}
	for i := range words {
		if rand.Float64() < rate {
			synonyms := []string{"effective", "optimal", "precise", "thorough", "concise"}
			words[i] = synonyms[rand.Intn(len(synonyms))]
			mutations = append(mutations, "word_swap")
		}
	}
	if rand.Float64() < rate {
		additions := []string{"Be specific.", "Think step by step.", "Provide examples."}
		words = append(words, additions[rand.Intn(len(additions))])
		mutations = append(mutations, "append_instruction")
	}
	return strings.Join(words, " "), mutations
}

// Select picks the top N genomes by fitness from a scored list.
func Select(fitnesses []float64, n int) []int {
	type scored struct{ idx int; fit float64 }
	items := make([]scored, len(fitnesses))
	for i, f := range fitnesses {
		items[i] = scored{i, f}
	}
	// Simple truncation selection
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if items[j].fit > items[i].fit {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	result := make([]int, 0, n)
	for i := 0; i < n && i < len(items); i++ {
		result = append(result, items[i].idx)
	}
	return result
}
