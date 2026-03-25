package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to write a temp YAML file and return its path.
func writeTempSpec(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validSpecYAML() string {
	return `
name: test-app
language: go
features:
  - name: auth
    behaviors:
      - name: login
        when: user submits valid credentials
        then:
          - return JWT token
        acceptance:
          - description: returns 200 with token
            method: POST
            path: /api/login
            expect:
              status: 200
`
}

func TestLoadSpec_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempSpec(t, dir, "app.spec.yaml", validSpecYAML())

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "test-app" {
		t.Errorf("name: got %q, want test-app", spec.Name)
	}
	if spec.Language != "go" {
		t.Errorf("language: got %q, want go", spec.Language)
	}
	if len(spec.Features) != 1 {
		t.Fatalf("features: got %d, want 1", len(spec.Features))
	}
	if spec.Features[0].Name != "auth" {
		t.Errorf("feature name: got %q, want auth", spec.Features[0].Name)
	}
	if len(spec.Features[0].Behaviors) != 1 {
		t.Fatalf("behaviors: got %d, want 1", len(spec.Features[0].Behaviors))
	}
	b := spec.Features[0].Behaviors[0]
	if b.Name != "login" {
		t.Errorf("behavior name: got %q, want login", b.Name)
	}
	if b.When != "user submits valid credentials" {
		t.Errorf("when: got %q", b.When)
	}
	if len(b.Then) != 1 || b.Then[0] != "return JWT token" {
		t.Errorf("then: got %v", b.Then)
	}
}

func TestLoadSpec_DefaultLanguage(t *testing.T) {
	dir := t.TempDir()
	yaml := `
name: myapp
features:
  - name: f1
    behaviors:
      - name: b1
        when: something
        then: [result]
        acceptance:
          - description: works
            expect: {}
`
	path := writeTempSpec(t, dir, "app.spec.yaml", yaml)
	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Language != "go" {
		t.Errorf("expected default language 'go', got %q", spec.Language)
	}
}

func TestLoadSpec_FileNotFound(t *testing.T) {
	_, err := LoadSpec("/nonexistent/path/foo.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadSpec_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeTempSpec(t, dir, "bad.spec.yaml", "}{not yaml at all")
	_, err := LoadSpec(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidation_Errors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing name",
			yaml:    "language: go\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        when: x\n        then: [y]\n        acceptance:\n          - description: d\n            expect: {}",
			wantErr: "name",
		},
		{
			name:    "unsupported language",
			yaml:    "name: app\nlanguage: rust\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        when: x\n        then: [y]\n        acceptance:\n          - description: d\n            expect: {}",
			wantErr: "unsupported language",
		},
		{
			name:    "no features",
			yaml:    "name: app\nlanguage: go",
			wantErr: "at least one feature",
		},
		{
			name:    "feature without name",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - behaviors:\n      - name: b\n        when: x\n        then: [y]\n        acceptance:\n          - description: d\n            expect: {}",
			wantErr: "must have a name",
		},
		{
			name:    "feature without behaviors",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - name: f\n    behaviors: []",
			wantErr: "at least one behavior",
		},
		{
			name:    "behavior without when",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        then: [y]\n        acceptance:\n          - description: d\n            expect: {}",
			wantErr: "when",
		},
		{
			name:    "behavior without then",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        when: x\n        acceptance:\n          - description: d\n            expect: {}",
			wantErr: "then",
		},
		{
			name:    "behavior without acceptance",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        when: x\n        then: [y]",
			wantErr: "acceptance",
		},
		{
			name:    "acceptance without description",
			yaml:    "name: app\nlanguage: go\nfeatures:\n  - name: f\n    behaviors:\n      - name: b\n        when: x\n        then: [y]\n        acceptance:\n          - expect: {}",
			wantErr: "description",
		},
		{
			name: "duplicate behavior name",
			yaml: `name: app
language: go
features:
  - name: f
    behaviors:
      - name: b
        when: x
        then: [y]
        acceptance:
          - description: d
            expect: {}
      - name: b
        when: x2
        then: [y2]
        acceptance:
          - description: d2
            expect: {}`,
			wantErr: "duplicate behavior",
		},
		{
			name: "duplicate data model",
			yaml: `name: app
language: go
features:
  - name: f
    behaviors:
      - name: b
        when: x
        then: [y]
        acceptance:
          - description: d
            expect: {}
data_model:
  - name: User
    fields:
      - name: id
        type: string
  - name: User
    fields:
      - name: id
        type: string`,
			wantErr: "duplicate data model",
		},
		{
			name: "model with no fields",
			yaml: `name: app
language: go
features:
  - name: f
    behaviors:
      - name: b
        when: x
        then: [y]
        acceptance:
          - description: d
            expect: {}
data_model:
  - name: User
    fields: []`,
			wantErr: "at least one field",
		},
		{
			name: "duplicate field in model",
			yaml: `name: app
language: go
features:
  - name: f
    behaviors:
      - name: b
        when: x
        then: [y]
        acceptance:
          - description: d
            expect: {}
data_model:
  - name: User
    fields:
      - name: email
        type: string
      - name: email
        type: string`,
			wantErr: "duplicate field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTempSpec(t, dir, "test.spec.yaml", tt.yaml)
			_, err := LoadSpec(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !containsStr(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	writeTempSpec(t, dir, "a.spec.yaml", `
name: app
language: go
features:
  - name: auth
    behaviors:
      - name: login
        when: creds
        then: [token]
        acceptance:
          - description: ok
            expect: {}
`)
	writeTempSpec(t, dir, "b.spec.yaml", `
name: app
language: go
features:
  - name: billing
    behaviors:
      - name: charge
        when: order placed
        then: [payment]
        acceptance:
          - description: charged
            expect: {}
`)
	// A non-spec file that should be ignored.
	writeTempSpec(t, dir, "readme.txt", "not a spec")

	spec, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Features) != 2 {
		t.Errorf("expected 2 features after merge, got %d", len(spec.Features))
	}
}

func TestLoadDir_NoSpecFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestLoadDir_NonexistentDir(t *testing.T) {
	_, err := LoadDir("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestGetStats(t *testing.T) {
	spec := &Spec{
		Name:     "test",
		Language: "go",
		Features: []Feature{
			{
				Name: "auth",
				Behaviors: []Behavior{
					{
						Name: "login",
						Acceptance: []Acceptance{
							{Description: "a1"},
							{Description: "a2"},
						},
					},
					{
						Name: "logout",
						Acceptance: []Acceptance{
							{Description: "a3"},
						},
					},
				},
			},
			{
				Name: "billing",
				Behaviors: []Behavior{
					{
						Name: "charge",
						Acceptance: []Acceptance{
							{Description: "a4"},
						},
					},
				},
			},
		},
		DataModel: []Model{
			{Name: "User", Fields: []Field{{Name: "id"}, {Name: "email"}, {Name: "name"}}},
			{Name: "Order", Fields: []Field{{Name: "id"}, {Name: "total"}}},
		},
	}

	stats := GetStats(spec)
	if stats.Features != 2 {
		t.Errorf("features: got %d, want 2", stats.Features)
	}
	if stats.Behaviors != 3 {
		t.Errorf("behaviors: got %d, want 3", stats.Behaviors)
	}
	if stats.Acceptance != 4 {
		t.Errorf("acceptance: got %d, want 4", stats.Acceptance)
	}
	if stats.Models != 2 {
		t.Errorf("models: got %d, want 2", stats.Models)
	}
	if stats.Fields != 5 {
		t.Errorf("fields: got %d, want 5", stats.Fields)
	}
}

func TestLoadSpec_WithDataModel(t *testing.T) {
	dir := t.TempDir()
	yaml := `
name: app
language: python
features:
  - name: users
    behaviors:
      - name: register
        when: signup form submitted
        then:
          - create user record
        acceptance:
          - description: user created
            method: POST
            path: /api/users
            body:
              email: test@example.com
            expect:
              status: 201
data_model:
  - name: User
    fields:
      - name: id
        type: string
        required: true
        unique: true
      - name: email
        type: string
        required: true
        validation: "format:email"
config:
  port: 8080
  storage: postgres
  auth: jwt
`
	path := writeTempSpec(t, dir, "app.spec.yaml", yaml)
	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Language != "python" {
		t.Errorf("language: got %q, want python", spec.Language)
	}
	if len(spec.DataModel) != 1 {
		t.Fatalf("models: got %d, want 1", len(spec.DataModel))
	}
	m := spec.DataModel[0]
	if m.Name != "User" {
		t.Errorf("model name: got %q", m.Name)
	}
	if len(m.Fields) != 2 {
		t.Fatalf("fields: got %d, want 2", len(m.Fields))
	}
	if !m.Fields[0].Required {
		t.Error("id field should be required")
	}
	if !m.Fields[0].Unique {
		t.Error("id field should be unique")
	}
	if spec.Config.Port != 8080 {
		t.Errorf("port: got %d, want 8080", spec.Config.Port)
	}
	if spec.Config.Storage != "postgres" {
		t.Errorf("storage: got %q", spec.Config.Storage)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
