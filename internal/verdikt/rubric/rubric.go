// Package rubric provides custom scoring rubrics that control how Verdikt
// weights each evaluation dimension. Rubrics are loaded from YAML files
// and can include domain-specific criteria injected into the LLM prompt.
package rubric

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Example is a few-shot example embedded in a rubric for calibration.
type Example struct {
	Prompt   string  `yaml:"prompt"   json:"prompt"`
	Response string  `yaml:"response" json:"response"`
	Score    float64 `yaml:"score"    json:"score"`
	Notes    string  `yaml:"notes"    json:"notes,omitempty"`
}

// Rubric defines domain-specific scoring weights and custom criteria.
type Rubric struct {
	Domain           string             `yaml:"domain"   json:"domain"`
	DimensionWeights map[string]float64 `yaml:"weights"  json:"weights"`
	CustomCriteria   []string           `yaml:"criteria" json:"criteria,omitempty"`
	Examples         []Example          `yaml:"examples" json:"examples,omitempty"`
}

// DefaultRubric returns the built-in equal-ish weight rubric used when no
// domain-specific rubric is loaded.
func DefaultRubric() *Rubric {
	return &Rubric{
		Domain: "default",
		DimensionWeights: map[string]float64{
			"relevance":   0.25,
			"accuracy":    0.25,
			"helpfulness": 0.20,
			"safety":      0.15,
			"tone":        0.15,
		},
	}
}

// LoadRubrics reads every .yaml / .yml file in dir and returns a map keyed
// by the rubric's domain name. Files that fail to parse are logged and skipped.
func LoadRubrics(dir string) (map[string]*Rubric, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read rubric dir %q: %w", dir, err)
	}

	out := make(map[string]*Rubric)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		var r Rubric
		if err := yaml.Unmarshal(data, &r); err != nil {
			continue
		}
		if r.Domain == "" {
			r.Domain = strings.TrimSuffix(e.Name(), ext)
		}
		// Normalise weights so they sum to 1.0.
		r.normaliseWeights()
		out[r.Domain] = &r
	}
	return out, nil
}

// WeightedScore computes the overall score from per-dimension scores using
// this rubric's weights.
func (r *Rubric) WeightedScore(dims map[string]float64) float64 {
	var sum, wsum float64
	for dim, w := range r.DimensionWeights {
		if v, ok := dims[dim]; ok {
			sum += v * w
			wsum += w
		}
	}
	if wsum == 0 {
		return 0
	}
	return sum / wsum
}

// CriteriaBlock returns a formatted string of custom criteria suitable for
// injection into the LLM evaluation prompt.
func (r *Rubric) CriteriaBlock() string {
	if len(r.CustomCriteria) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("CUSTOM CRITERIA (must be considered):\n")
	for i, c := range r.CustomCriteria {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, c))
	}
	return sb.String()
}

// normaliseWeights adjusts weights so they sum to 1.0.
func (r *Rubric) normaliseWeights() {
	var total float64
	for _, w := range r.DimensionWeights {
		total += w
	}
	if total == 0 || total == 1.0 {
		return
	}
	for k, w := range r.DimensionWeights {
		r.DimensionWeights[k] = w / total
	}
}
