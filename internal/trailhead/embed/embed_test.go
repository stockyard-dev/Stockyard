package embed

import (
	"testing"
)

func TestNew_Defaults(t *testing.T) {
	g := New(Config{APIKey: "test-key"})
	if g.cfg.Model != "text-embedding-3-small" {
		t.Errorf("default Model = %q, want %q", g.cfg.Model, "text-embedding-3-small")
	}
	if g.cfg.BaseURL != "https://api.openai.com" {
		t.Errorf("default BaseURL = %q, want %q", g.cfg.BaseURL, "https://api.openai.com")
	}
	if g.cfg.Dimensions != 1536 {
		t.Errorf("default Dimensions = %d, want 1536", g.cfg.Dimensions)
	}
	if g.cfg.BatchSize != 50 {
		t.Errorf("default BatchSize = %d, want 50", g.cfg.BatchSize)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	g := New(Config{
		APIKey:     "key",
		Model:      "custom-model",
		BaseURL:    "https://custom.api.com",
		Dimensions: 768,
		BatchSize:  25,
	})
	if g.cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", g.cfg.Model, "custom-model")
	}
	if g.cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want %q", g.cfg.BaseURL, "https://custom.api.com")
	}
	if g.cfg.Dimensions != 768 {
		t.Errorf("Dimensions = %d, want 768", g.cfg.Dimensions)
	}
	if g.cfg.BatchSize != 25 {
		t.Errorf("BatchSize = %d, want 25", g.cfg.BatchSize)
	}
}

func TestNew_ZeroBatchSize(t *testing.T) {
	g := New(Config{APIKey: "key", BatchSize: 0})
	if g.cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50 (default)", g.cfg.BatchSize)
	}
}

func TestNew_NegativeBatchSize(t *testing.T) {
	g := New(Config{APIKey: "key", BatchSize: -1})
	if g.cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50 (default)", g.cfg.BatchSize)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"exact", 5, "exact"},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
