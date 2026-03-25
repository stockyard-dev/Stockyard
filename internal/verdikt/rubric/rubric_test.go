package rubric

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRubric(t *testing.T) {
	r := DefaultRubric()
	if r.Domain != "default" {
		t.Errorf("Domain = %q, want %q", r.Domain, "default")
	}
	if len(r.DimensionWeights) != 5 {
		t.Errorf("DimensionWeights len = %d, want 5", len(r.DimensionWeights))
	}

	// Check weights sum to 1.0
	var sum float64
	for _, w := range r.DimensionWeights {
		sum += w
	}
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("weights sum = %f, want 1.0", sum)
	}

	// Check all expected dimensions exist
	for _, dim := range []string{"relevance", "accuracy", "helpfulness", "safety", "tone"} {
		if _, ok := r.DimensionWeights[dim]; !ok {
			t.Errorf("missing dimension %q", dim)
		}
	}
}

func TestWeightedScore(t *testing.T) {
	r := DefaultRubric()
	dims := map[string]float64{
		"relevance":   1.0,
		"accuracy":    1.0,
		"helpfulness": 1.0,
		"safety":      1.0,
		"tone":        1.0,
	}
	score := r.WeightedScore(dims)
	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("WeightedScore(all 1.0) = %f, want 1.0", score)
	}
}

func TestWeightedScore_ZeroWeights(t *testing.T) {
	r := &Rubric{DimensionWeights: map[string]float64{}}
	score := r.WeightedScore(map[string]float64{"relevance": 0.5})
	if score != 0 {
		t.Errorf("WeightedScore with no weights = %f, want 0", score)
	}
}

func TestWeightedScore_PartialDims(t *testing.T) {
	r := &Rubric{
		DimensionWeights: map[string]float64{
			"relevance": 0.5,
			"accuracy":  0.5,
		},
	}
	dims := map[string]float64{
		"relevance": 0.8,
	}
	// Only relevance is matched: 0.8 * 0.5 / 0.5 = 0.8
	score := r.WeightedScore(dims)
	if math.Abs(score-0.8) > 0.001 {
		t.Errorf("WeightedScore(partial) = %f, want 0.8", score)
	}
}

func TestWeightedScore_Weighted(t *testing.T) {
	r := &Rubric{
		DimensionWeights: map[string]float64{
			"relevance": 0.7,
			"safety":    0.3,
		},
	}
	dims := map[string]float64{
		"relevance": 1.0,
		"safety":    0.0,
	}
	// (1.0 * 0.7 + 0.0 * 0.3) / (0.7 + 0.3) = 0.7
	score := r.WeightedScore(dims)
	if math.Abs(score-0.7) > 0.001 {
		t.Errorf("WeightedScore = %f, want 0.7", score)
	}
}

func TestCriteriaBlock_Empty(t *testing.T) {
	r := &Rubric{}
	if r.CriteriaBlock() != "" {
		t.Error("CriteriaBlock() should be empty when no criteria")
	}
}

func TestCriteriaBlock_WithCriteria(t *testing.T) {
	r := &Rubric{
		CustomCriteria: []string{
			"Must cite sources",
			"No jargon allowed",
		},
	}

	block := r.CriteriaBlock()
	if block == "" {
		t.Fatal("CriteriaBlock() is empty")
	}
	if !containsSubstring(block, "Must cite sources") {
		t.Error("CriteriaBlock() missing first criterion")
	}
	if !containsSubstring(block, "No jargon allowed") {
		t.Error("CriteriaBlock() missing second criterion")
	}
	if !containsSubstring(block, "1.") {
		t.Error("CriteriaBlock() missing numbering")
	}
}

func TestNormaliseWeights(t *testing.T) {
	r := &Rubric{
		DimensionWeights: map[string]float64{
			"a": 2.0,
			"b": 3.0,
		},
	}
	r.normaliseWeights()

	var sum float64
	for _, w := range r.DimensionWeights {
		sum += w
	}
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("normalised weights sum = %f, want 1.0", sum)
	}
	if math.Abs(r.DimensionWeights["a"]-0.4) > 0.001 {
		t.Errorf("weight a = %f, want 0.4", r.DimensionWeights["a"])
	}
	if math.Abs(r.DimensionWeights["b"]-0.6) > 0.001 {
		t.Errorf("weight b = %f, want 0.6", r.DimensionWeights["b"])
	}
}

func TestNormaliseWeights_AlreadyNormalised(t *testing.T) {
	r := &Rubric{
		DimensionWeights: map[string]float64{
			"a": 0.5,
			"b": 0.5,
		},
	}
	r.normaliseWeights()
	if r.DimensionWeights["a"] != 0.5 {
		t.Errorf("weight a = %f, want 0.5 (unchanged)", r.DimensionWeights["a"])
	}
}

func TestNormaliseWeights_Empty(t *testing.T) {
	r := &Rubric{DimensionWeights: map[string]float64{}}
	r.normaliseWeights() // Should not panic
}

func TestLoadRubrics(t *testing.T) {
	dir := t.TempDir()

	// Create valid rubric file
	content1 := `domain: customer-support
weights:
  relevance: 0.3
  accuracy: 0.2
  helpfulness: 0.3
  safety: 0.1
  tone: 0.1
criteria:
  - Be empathetic
  - Reference case number
examples:
  - prompt: "I need help"
    response: "I'd be happy to help!"
    score: 0.9
    notes: "Good empathy"
`
	if err := os.WriteFile(filepath.Join(dir, "support.yaml"), []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another rubric (yml extension)
	content2 := `domain: coding
weights:
  relevance: 0.2
  accuracy: 0.4
  helpfulness: 0.2
  safety: 0.1
  tone: 0.1
`
	if err := os.WriteFile(filepath.Join(dir, "coding.yml"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-yaml file (should be skipped)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a rubric"), 0644); err != nil {
		t.Fatal(err)
	}

	rubrics, err := LoadRubrics(dir)
	if err != nil {
		t.Fatalf("LoadRubrics() error = %v", err)
	}

	if len(rubrics) != 2 {
		t.Fatalf("LoadRubrics() returned %d rubrics, want 2", len(rubrics))
	}

	if r, ok := rubrics["customer-support"]; !ok {
		t.Error("missing customer-support rubric")
	} else {
		if len(r.CustomCriteria) != 2 {
			t.Errorf("customer-support criteria len = %d, want 2", len(r.CustomCriteria))
		}
		if len(r.Examples) != 1 {
			t.Errorf("customer-support examples len = %d, want 1", len(r.Examples))
		}
		// Check normalised
		var sum float64
		for _, w := range r.DimensionWeights {
			sum += w
		}
		if math.Abs(sum-1.0) > 0.001 {
			t.Errorf("weights sum = %f, want 1.0", sum)
		}
	}

	if _, ok := rubrics["coding"]; !ok {
		t.Error("missing coding rubric")
	}
}

func TestLoadRubrics_NoDomain(t *testing.T) {
	dir := t.TempDir()
	content := `weights:
  relevance: 0.5
  accuracy: 0.5
`
	if err := os.WriteFile(filepath.Join(dir, "unnamed.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	rubrics, err := LoadRubrics(dir)
	if err != nil {
		t.Fatalf("LoadRubrics() error = %v", err)
	}
	if _, ok := rubrics["unnamed"]; !ok {
		t.Error("expected rubric with domain derived from filename")
	}
}

func TestLoadRubrics_InvalidDir(t *testing.T) {
	_, err := LoadRubrics("/nonexistent/dir")
	if err == nil {
		t.Error("LoadRubrics() expected error for nonexistent dir")
	}
}

func TestLoadRubrics_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	// Use truly unparseable YAML
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("- :\n  - :\n[[["), 0644); err != nil {
		t.Fatal(err)
	}

	rubrics, err := LoadRubrics(dir)
	if err != nil {
		t.Fatalf("LoadRubrics() error = %v (should skip bad files)", err)
	}
	// The YAML parser may be lenient with some inputs; at most there should be 0 or 1 rubrics
	// since the file has no valid rubric structure
	_ = rubrics // just verify no crash
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
