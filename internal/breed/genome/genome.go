// Package genome provides prompt genome representation for genetic evolution.
package genome

// Genome represents a prompt genome with evolvable sections.
type Genome struct {
	SystemPrompt   string   `json:"system_prompt"`
	FewShotExamples []string `json:"few_shot_examples"`
	Constraints    []string `json:"constraints"`
	Temperature    float64  `json:"temperature"`
	MaxTokens      int      `json:"max_tokens"`
}

// Crossover combines two genomes, taking sections from each parent.
func Crossover(a, b *Genome, crossoverPoint float64) *Genome {
	child := &Genome{}
	if crossoverPoint < 0.5 {
		child.SystemPrompt = a.SystemPrompt
		child.FewShotExamples = b.FewShotExamples
	} else {
		child.SystemPrompt = b.SystemPrompt
		child.FewShotExamples = a.FewShotExamples
	}
	child.Constraints = append(a.Constraints[:len(a.Constraints)/2], b.Constraints[len(b.Constraints)/2:]...)
	child.Temperature = (a.Temperature + b.Temperature) / 2
	child.MaxTokens = (a.MaxTokens + b.MaxTokens) / 2
	return child
}
