// Package config handles Tide YAML configuration loading.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents a tide.yaml configuration file.
type Config struct {
	Threshold float64          `yaml:"threshold"`
	Features  []FeatureConfig  `yaml:"features"`
}

// FeatureConfig describes a feature loaded from configuration.
type FeatureConfig struct {
	ID        string  `yaml:"id"`
	Name      string  `yaml:"name"`
	Priority  int     `yaml:"priority"`
	CPUWeight float64 `yaml:"cpu_weight"`
	MemWeight float64 `yaml:"mem_weight"`
	Fallback  string  `yaml:"fallback"`
}

// LoadConfig reads and parses a tide.yaml configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Threshold <= 0 || cfg.Threshold > 1 {
		cfg.Threshold = 0.7
	}
	return &cfg, nil
}
