package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tide.yaml")

	content := `threshold: 0.8
features:
  - id: feat-1
    name: Feature One
    priority: 1
    cpu_weight: 0.5
    mem_weight: 0.3
    fallback: default
  - id: feat-2
    name: Feature Two
    priority: 2
    cpu_weight: 0.7
    mem_weight: 0.6
    fallback: none
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Threshold != 0.8 {
		t.Errorf("Threshold = %f, want 0.8", cfg.Threshold)
	}
	if len(cfg.Features) != 2 {
		t.Fatalf("Features len = %d, want 2", len(cfg.Features))
	}
	if cfg.Features[0].ID != "feat-1" {
		t.Errorf("Features[0].ID = %q, want %q", cfg.Features[0].ID, "feat-1")
	}
	if cfg.Features[0].Name != "Feature One" {
		t.Errorf("Features[0].Name = %q, want %q", cfg.Features[0].Name, "Feature One")
	}
	if cfg.Features[0].CPUWeight != 0.5 {
		t.Errorf("Features[0].CPUWeight = %f, want 0.5", cfg.Features[0].CPUWeight)
	}
	if cfg.Features[1].Fallback != "none" {
		t.Errorf("Features[1].Fallback = %q, want %q", cfg.Features[1].Fallback, "none")
	}
}

func TestLoadConfig_DefaultThreshold(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantThres float64
	}{
		{"zero threshold", "threshold: 0\n", 0.7},
		{"negative threshold", "threshold: -1\n", 0.7},
		{"threshold > 1", "threshold: 1.5\n", 0.7},
		{"valid threshold", "threshold: 0.9\n", 0.9},
		{"threshold exactly 1", "threshold: 1.0\n", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tide.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if cfg.Threshold != tt.wantThres {
				t.Errorf("Threshold = %f, want %f", cfg.Threshold, tt.wantThres)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/tide.yaml")
	if err == nil {
		t.Error("LoadConfig() expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("threshold:\n  - bad\n  - [unclosed"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("LoadConfig() expected error for invalid YAML")
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	// Should default threshold
	if cfg.Threshold != 0.7 {
		t.Errorf("Threshold = %f, want 0.7 (default)", cfg.Threshold)
	}
	if len(cfg.Features) != 0 {
		t.Errorf("Features len = %d, want 0", len(cfg.Features))
	}
}
