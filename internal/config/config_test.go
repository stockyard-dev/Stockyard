package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	products := []struct {
		name string
		port int
	}{
		{"costcap", 4100},
		{"llmcache", 4101},
		{"jsonguard", 4102},
		{"routefall", 4103},
		{"rateshield", 4104},
		{"promptreplay", 4105},
		{"stockyard", 4200},
	}

	for _, tt := range products {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(tt.name)
			if cfg.Port != tt.port {
				t.Errorf("port = %d, want %d", cfg.Port, tt.port)
			}
			if cfg.Product != tt.name {
				t.Errorf("product = %q, want %q", cfg.Product, tt.name)
			}
			if cfg.Providers["openai"].APIKey != "${OPENAI_API_KEY}" {
				t.Error("openai api key template not set")
			}
		})
	}
}

func TestDefaultConfigFeatures(t *testing.T) {
	// CostCap: no body storage
	cc := DefaultConfig("costcap")
	if cc.Logging.StoreBodies {
		t.Error("costcap should not store bodies by default")
	}
	if cc.Projects["default"].Caps.Daily != 5.00 {
		t.Errorf("costcap daily cap = %f, want 5.00", cc.Projects["default"].Caps.Daily)
	}

	// CacheLayer: cache enabled
	cl := DefaultConfig("llmcache")
	if !cl.Cache.Enabled {
		t.Error("llmcache should have cache enabled")
	}
	if cl.Cache.TTL.Duration != 1*time.Hour {
		t.Errorf("llmcache cache TTL = %v, want 1h", cl.Cache.TTL.Duration)
	}

	// FallbackRouter: failover enabled
	fr := DefaultConfig("routefall")
	if !fr.Failover.Enabled {
		t.Error("routefall should have failover enabled")
	}
	if len(fr.Failover.Providers) != 3 {
		t.Errorf("routefall failover providers = %d, want 3", len(fr.Failover.Providers))
	}

	// RateShield: rate limiting enabled
	rs := DefaultConfig("rateshield")
	if !rs.RateLimit.Enabled {
		t.Error("rateshield should have rate limiting enabled")
	}

	// Stockyard: everything enabled
	lk := DefaultConfig("stockyard")
	if !lk.Cache.Enabled || !lk.Failover.Enabled || !lk.RateLimit.Enabled {
		t.Error("stockyard should have all features enabled")
	}
}

func TestEnvVarInterpolation(t *testing.T) {
	// Create a temp config file with env var
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	os.Setenv("TEST_LLM_KEY", "sk-test-12345")
	defer os.Unsetenv("TEST_LLM_KEY")

	content := `{
		"port": 4100,
		"product": "costcap",
		"data_dir": "/tmp/stockyard-test",
		"providers": {
			"openai": {
				"api_key": "${TEST_LLM_KEY}"
			}
		}
	}`
	tmpFile.WriteString(content)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Providers["openai"].APIKey != "sk-test-12345" {
		t.Errorf("api_key = %q, want sk-test-12345", cfg.Providers["openai"].APIKey)
	}
	if cfg.Port != 4100 {
		t.Errorf("port = %d, want 4100", cfg.Port)
	}
}

func TestValidate(t *testing.T) {
	// Missing port
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing port")
	}

	// Valid config
	cfg = &Config{Port: 4100}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
