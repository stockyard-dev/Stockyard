// Package parser reads and validates Stockyard Spec files.
//
// A spec file defines what software should do, not how. Each behavior
// is a declarative statement with acceptance criteria. The spec is the
// source of truth — code is a compiled artifact generated from it.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec represents a complete software specification.
type Spec struct {
	Name        string       `yaml:"name" json:"name"`
	Description string       `yaml:"description" json:"description"`
	Language    string       `yaml:"language" json:"language"` // go, python, typescript
	Framework   string       `yaml:"framework,omitempty" json:"framework,omitempty"`
	Features    []Feature    `yaml:"features" json:"features"`
	DataModel   []Model      `yaml:"data_model,omitempty" json:"data_model,omitempty"`
	Config      SpecConfig   `yaml:"config,omitempty" json:"config,omitempty"`
}

// Feature is a named group of related behaviors.
type Feature struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Behaviors   []Behavior `yaml:"behaviors" json:"behaviors"`
}

// Behavior is the atomic unit of specification — a single thing the software does.
type Behavior struct {
	Name        string       `yaml:"name" json:"name"`
	When        string       `yaml:"when" json:"when"`                             // trigger condition
	Then        []string     `yaml:"then" json:"then"`                             // expected outcomes
	Constraints []string     `yaml:"constraints,omitempty" json:"constraints,omitempty"` // rules that must hold
	Acceptance  []Acceptance `yaml:"acceptance" json:"acceptance"`                  // testable criteria
	DependsOn   []string     `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// Acceptance defines a single testable criterion for a behavior.
type Acceptance struct {
	Description string            `yaml:"description" json:"description"`
	Method      string            `yaml:"method,omitempty" json:"method,omitempty"`   // GET, POST, PUT, DELETE
	Path        string            `yaml:"path,omitempty" json:"path,omitempty"`       // /api/users
	Body        map[string]any    `yaml:"body,omitempty" json:"body,omitempty"`       // request body
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Expect      Expectation       `yaml:"expect" json:"expect"`
}

// Expectation defines what the acceptance test should verify.
type Expectation struct {
	Status  int               `yaml:"status,omitempty" json:"status,omitempty"`     // HTTP status code
	Body    map[string]any    `yaml:"body,omitempty" json:"body,omitempty"`         // response body assertions
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Contains []string         `yaml:"contains,omitempty" json:"contains,omitempty"` // response body contains these strings
	NotContains []string      `yaml:"not_contains,omitempty" json:"not_contains,omitempty"`
}

// Model defines a data entity in the specification.
type Model struct {
	Name   string  `yaml:"name" json:"name"`
	Fields []Field `yaml:"fields" json:"fields"`
}

// Field defines a single field in a data model.
type Field struct {
	Name       string `yaml:"name" json:"name"`
	Type       string `yaml:"type" json:"type"`             // string, integer, boolean, timestamp, float, text
	Required   bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Unique     bool   `yaml:"unique,omitempty" json:"unique,omitempty"`
	Indexed    bool   `yaml:"indexed,omitempty" json:"indexed,omitempty"`
	Default    string `yaml:"default,omitempty" json:"default,omitempty"`
	Validation string `yaml:"validation,omitempty" json:"validation,omitempty"` // min_length:12, format:email, etc.
}

// SpecConfig holds project-level configuration.
type SpecConfig struct {
	Port        int               `yaml:"port,omitempty" json:"port,omitempty"`
	Storage     string            `yaml:"storage,omitempty" json:"storage,omitempty"` // sqlite, postgres
	Auth        string            `yaml:"auth,omitempty" json:"auth,omitempty"`       // none, jwt, api_key
	Integrations map[string]string `yaml:"integrations,omitempty" json:"integrations,omitempty"` // email: smtp, etc.
}

// LoadSpec reads a single spec file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file %s: %w", path, err)
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing spec file %s: %w", path, err)
	}
	if err := validate(&spec); err != nil {
		return nil, fmt.Errorf("validating spec %s: %w", path, err)
	}
	return &spec, nil
}

// LoadDir reads all .spec.yaml files from a directory and merges them.
func LoadDir(dir string) (*Spec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading spec directory %s: %w", dir, err)
	}

	var merged Spec
	fileCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".spec.yaml") && !strings.HasSuffix(name, ".spec.yml") {
			continue
		}

		spec, err := LoadSpec(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}

		if fileCount == 0 {
			merged = *spec
		} else {
			// Merge features and data models
			merged.Features = append(merged.Features, spec.Features...)
			merged.DataModel = append(merged.DataModel, spec.DataModel...)
		}
		fileCount++
	}

	if fileCount == 0 {
		return nil, fmt.Errorf("no .spec.yaml files found in %s", dir)
	}

	return &merged, nil
}

// validate checks a spec for structural errors.
func validate(spec *Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("spec 'name' is required")
	}
	if spec.Language == "" {
		spec.Language = "go" // default
	}
	if spec.Language != "go" && spec.Language != "python" && spec.Language != "typescript" {
		return fmt.Errorf("unsupported language %q (supported: go, python, typescript)", spec.Language)
	}

	if len(spec.Features) == 0 {
		return fmt.Errorf("spec must have at least one feature")
	}

	behaviorNames := map[string]bool{}
	for fi, feature := range spec.Features {
		if feature.Name == "" {
			return fmt.Errorf("feature[%d] must have a name", fi)
		}
		if len(feature.Behaviors) == 0 {
			return fmt.Errorf("feature %q must have at least one behavior", feature.Name)
		}

		for bi, behavior := range feature.Behaviors {
			if behavior.Name == "" {
				return fmt.Errorf("feature %q behavior[%d] must have a name", feature.Name, bi)
			}
			fullName := feature.Name + "." + behavior.Name
			if behaviorNames[fullName] {
				return fmt.Errorf("duplicate behavior name: %s", fullName)
			}
			behaviorNames[fullName] = true

			if behavior.When == "" {
				return fmt.Errorf("behavior %q must have a 'when' clause", fullName)
			}
			if len(behavior.Then) == 0 {
				return fmt.Errorf("behavior %q must have at least one 'then' outcome", fullName)
			}
			if len(behavior.Acceptance) == 0 {
				return fmt.Errorf("behavior %q must have at least one acceptance criterion", fullName)
			}

			for ai, acc := range behavior.Acceptance {
				if acc.Description == "" {
					return fmt.Errorf("behavior %q acceptance[%d] must have a description", fullName, ai)
				}
			}
		}
	}

	// Validate data model
	modelNames := map[string]bool{}
	for _, model := range spec.DataModel {
		if model.Name == "" {
			return fmt.Errorf("data model must have a name")
		}
		if modelNames[model.Name] {
			return fmt.Errorf("duplicate data model: %s", model.Name)
		}
		modelNames[model.Name] = true

		if len(model.Fields) == 0 {
			return fmt.Errorf("data model %q must have at least one field", model.Name)
		}
		fieldNames := map[string]bool{}
		for _, field := range model.Fields {
			if field.Name == "" {
				return fmt.Errorf("model %q: field must have a name", model.Name)
			}
			if fieldNames[field.Name] {
				return fmt.Errorf("model %q: duplicate field %q", model.Name, field.Name)
			}
			fieldNames[field.Name] = true
		}
	}

	return nil
}

// Stats returns summary statistics about a spec.
type Stats struct {
	Features   int `json:"features"`
	Behaviors  int `json:"behaviors"`
	Acceptance int `json:"acceptance_criteria"`
	Models     int `json:"data_models"`
	Fields     int `json:"fields"`
}

// GetStats computes statistics for a spec.
func GetStats(spec *Spec) Stats {
	s := Stats{
		Features: len(spec.Features),
		Models:   len(spec.DataModel),
	}
	for _, f := range spec.Features {
		s.Behaviors += len(f.Behaviors)
		for _, b := range f.Behaviors {
			s.Acceptance += len(b.Acceptance)
		}
	}
	for _, m := range spec.DataModel {
		s.Fields += len(m.Fields)
	}
	return s
}
