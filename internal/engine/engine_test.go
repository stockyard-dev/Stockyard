package engine

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/dashboard"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/storage"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

func TestBuildMiddlewares(t *testing.T) {
	// Test that each product builds its middleware chain without panics
	products := []ProductConfig{
		{Name: "CostCap", Product: "costcap", Features: Features{SpendTracking: true, SpendCaps: true, Alerts: true, RequestLogging: true}},
		{Name: "CacheLayer", Product: "llmcache", Features: Features{Cache: true, SpendTracking: true, RequestLogging: true, FullBodyLog: true}},
		{Name: "StructuredShield", Product: "jsonguard", Features: Features{Validation: true, RequestLogging: true, FullBodyLog: true}},
		{Name: "FallbackRouter", Product: "routefall", Features: Features{Failover: true, RequestLogging: true, SpendTracking: true}},
		{Name: "RateShield", Product: "rateshield", Features: Features{RateLimiting: true, RequestLogging: true}},
		{Name: "PromptReplay", Product: "promptreplay", Features: Features{RequestLogging: true, FullBodyLog: true}},
		{Name: "Stockyard", Product: "stockyard", Features: Features{SpendTracking: true, SpendCaps: true, Alerts: true, Cache: true, Validation: true, Failover: true, RateLimiting: true, RequestLogging: true, FullBodyLog: true}},
	}

	for _, pc := range products {
		t.Run(pc.Name, func(t *testing.T) {
			cfg := config.DefaultConfig(pc.Product)
			counter := tracker.NewSpendCounter()
			broadcaster := dashboard.NewBroadcaster()
			providers := map[string]provider.Provider{}

			mw := buildMiddlewares(pc, cfg, nil, counter, broadcaster, providers)
			if len(mw) == 0 {
				t.Error("expected at least one middleware")
			}
			t.Logf("%s: %d middlewares built", pc.Name, len(mw))
		})
	}
}

func TestIsTemplate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"${OPENAI_API_KEY}", true},
		{"sk-real-key-12345", false},
		{"", false},
		{"${}", false},
		{"$OPENAI", false},
	}
	for _, tt := range tests {
		if got := isTemplate(tt.input); got != tt.want {
			t.Errorf("isTemplate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestInitProvidersNoKeys(t *testing.T) {
	cfg := config.DefaultConfig("costcap")
	// Default config has template keys like ${OPENAI_API_KEY}
	// Without env vars set, no providers should be created
	providers := initProviders(cfg)
	if len(providers) != 0 {
		t.Errorf("expected 0 providers with template keys, got %d", len(providers))
	}
}

func TestMakeSendHandlerNoProviders(t *testing.T) {
	handler := makeSendHandler(map[string]provider.Provider{})
	_, err := handler(nil, &provider.Request{Model: "gpt-4o-mini", Messages: []provider.Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Error("expected error with no providers configured")
	}
}

func TestBuildCaps(t *testing.T) {
	cfg := config.DefaultConfig("costcap")
	caps := buildCaps(cfg)

	if _, ok := caps["default"]; !ok {
		t.Fatal("expected 'default' project caps")
	}
	if caps["default"].DailyCap != 5.00 {
		t.Errorf("daily cap = %f, want 5.00", caps["default"].DailyCap)
	}
	if caps["default"].MonthlyCap != 50.00 {
		t.Errorf("monthly cap = %f, want 50.00", caps["default"].MonthlyCap)
	}
}

// Verify the logging middleware doesn't panic with a nil DB.
func TestLoggingMiddlewareNilDB(t *testing.T) {
	products := []ProductConfig{
		{Name: "PromptReplay", Product: "promptreplay", Features: Features{RequestLogging: true, FullBodyLog: true}},
	}
	cfg := config.DefaultConfig("promptreplay")
	counter := tracker.NewSpendCounter()
	broadcaster := dashboard.NewBroadcaster()

	// Pass nil DB — should not panic
	mw := buildMiddlewares(products[0], cfg, nil, counter, broadcaster, map[string]provider.Provider{})
	if len(mw) == 0 {
		t.Error("expected middlewares even with nil DB")
	}
}

// Verify the DB type satisfies the SpendStore interface.
func TestDBImplementsSpendStore(t *testing.T) {
	var _ tracker.SpendStore = (*storage.DB)(nil)
}
