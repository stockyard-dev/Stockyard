// Package persona handles loading, parsing, and managing synthetic user personas.
package persona

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

// Persona defines a synthetic user's character, goal, and behavioral traits.
type Persona struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Goal        string   `yaml:"goal" json:"goal"`
	Behavior    Behavior `yaml:"behavior" json:"behavior"`
}

// Behavior controls how a synthetic user interacts with the target application.
type Behavior struct {
	Patience             string  `yaml:"patience" json:"patience"`                           // low, medium, high
	TypoRate             float64 `yaml:"typo_rate" json:"typo_rate"`                         // 0.0 - 1.0
	ReadingComprehension string  `yaml:"reading_comprehension" json:"reading_comprehension"` // partial, full
	Specificity          string  `yaml:"specificity" json:"specificity"`                     // low, medium, high
	FollowUpDepth        string  `yaml:"follow_up_depth" json:"follow_up_depth"`             // shallow, medium, deep
	Escalation           string  `yaml:"escalation" json:"escalation"`                       // none, mild, aggressive
	AvgTurns             int     `yaml:"avg_turns" json:"avg_turns"`
	Techniques           []string `yaml:"techniques,omitempty" json:"techniques,omitempty"` // for adversarial personas
}

// Load loads a persona by name. Checks built-in personas first, then custom files.
func Load(name string) (Persona, error) {
	// Try built-in
	data, err := builtinFS.ReadFile("builtin/" + name + ".yaml")
	if err == nil {
		var p Persona
		if err := yaml.Unmarshal(data, &p); err != nil {
			return Persona{}, fmt.Errorf("parsing builtin persona %s: %w", name, err)
		}
		return p, nil
	}

	// Try as a file path
	if _, statErr := os.Stat(name); statErr == nil {
		return LoadFile(name)
	}

	// Try with .yaml extension
	if _, statErr := os.Stat(name + ".yaml"); statErr == nil {
		return LoadFile(name + ".yaml")
	}

	return Persona{}, fmt.Errorf("persona %q not found (checked built-in and filesystem)", name)
}

// LoadFile loads a persona from a YAML file path.
func LoadFile(path string) (Persona, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Persona{}, fmt.Errorf("reading persona file %s: %w", path, err)
	}
	var p Persona
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Persona{}, fmt.Errorf("parsing persona file %s: %w", path, err)
	}
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(path), ".yaml")
	}
	return p, nil
}

// BuiltinNames returns the names of all built-in personas in sorted order.
func BuiltinNames() []string {
	entries, _ := builtinFS.ReadDir("builtin")
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// LoadAll loads all built-in personas.
func LoadAll() ([]Persona, error) {
	var out []Persona
	for _, name := range BuiltinNames() {
		p, err := Load(name)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
