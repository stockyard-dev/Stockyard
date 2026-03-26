// Package evolve provides genetic operators for prompt evolution.
package evolve

import (
	"math/rand"
	"strings"
)

// Mutate applies prompt-aware mutations to a system prompt.
// Instead of random word swaps (which create gibberish), mutations
// add, remove, or rephrase instructions while keeping the core intact.
func Mutate(prompt string, rate float64) (string, []string) {
	var mutations []string

	// Tone shifts — prepend a tone modifier
	if rand.Float64() < rate {
		tones := []string{
			"Be extremely concise. ",
			"Use vivid language. ",
			"Be direct and punchy. ",
			"Channel confidence. ",
			"Think like a developer. ",
		}
		prompt = tones[rand.Intn(len(tones))] + prompt
		mutations = append(mutations, "tone_shift")
	}

	// Constraint injection — append a constraint
	if rand.Float64() < rate {
		constraints := []string{
			" Keep it under 8 words.",
			" Make it sound like a Unix philosophy.",
			" Use an action verb to start.",
			" Include a metaphor.",
			" Make it rhyme or have rhythm.",
			" Focus on the single-binary angle.",
		}
		prompt += constraints[rand.Intn(len(constraints))]
		mutations = append(mutations, "constraint_add")
	}

	// Sentence shuffle — reorder sentences if multiple exist
	sentences := strings.Split(prompt, ". ")
	if len(sentences) > 2 && rand.Float64() < rate {
		i := rand.Intn(len(sentences) - 1)
		sentences[i], sentences[i+1] = sentences[i+1], sentences[i]
		prompt = strings.Join(sentences, ". ")
		mutations = append(mutations, "sentence_shuffle")
	}

	// Trimming — remove a random sentence to keep prompts from bloating
	if len(sentences) > 3 && rand.Float64() < rate*0.5 {
		remove := 1 + rand.Intn(len(sentences)-2) // don't remove first or last
		sentences = append(sentences[:remove], sentences[remove+1:]...)
		prompt = strings.Join(sentences, ". ")
		mutations = append(mutations, "trim")
	}

	return prompt, mutations
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
