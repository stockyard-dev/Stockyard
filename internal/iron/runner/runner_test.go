package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/iron/codegen"
	"github.com/stockyard-dev/stockyard/internal/iron/parser"
)

func TestTestFilePath(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"go", filepath.Join("out", "spec_test.go")},
		{"python", filepath.Join("out", "test_spec.py")},
		{"typescript", filepath.Join("out", "spec.test.ts")},
		{"", filepath.Join("out", "spec_test.go")}, // default
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			spec := &parser.Spec{Language: tt.language}
			got := testFilePath(spec, "out")
			if got != tt.want {
				t.Errorf("testFilePath(%q) = %q, want %q", tt.language, got, tt.want)
			}
		})
	}
}

func TestCountAcceptance(t *testing.T) {
	spec := &parser.Spec{
		Features: []parser.Feature{
			{
				Behaviors: []parser.Behavior{
					{Acceptance: []parser.Acceptance{{}, {}}},
					{Acceptance: []parser.Acceptance{{}}},
				},
			},
			{
				Behaviors: []parser.Behavior{
					{Acceptance: []parser.Acceptance{{}, {}, {}}},
				},
			},
		},
	}

	got := countAcceptance(spec)
	if got != 6 {
		t.Errorf("expected 6 acceptance criteria, got %d", got)
	}
}

func TestCountAcceptance_Empty(t *testing.T) {
	spec := &parser.Spec{}
	if countAcceptance(spec) != 0 {
		t.Error("expected 0 for empty spec")
	}
}

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()

	files := []codegen.File{
		{Path: "main.go", Content: "package main"},
		{Path: "sub/helper.go", Content: "package sub"},
	}

	writeFiles(files, dir)

	for _, f := range files {
		path := filepath.Join(dir, f.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", f.Path, err)
			continue
		}
		if string(data) != f.Content {
			t.Errorf("file %s: got %q, want %q", f.Path, string(data), f.Content)
		}
	}
}

func TestWriteFiles_CreatesSubdirs(t *testing.T) {
	dir := t.TempDir()

	files := []codegen.File{
		{Path: "deep/nested/dir/file.go", Content: "package deep"},
	}

	writeFiles(files, dir)

	path := filepath.Join(dir, "deep", "nested", "dir", "file.go")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected nested directory structure to be created")
	}
}
