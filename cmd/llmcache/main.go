// CacheLayer — "Cut your LLM costs by 30%+ with one line."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "CacheLayer",
		Product: "llmcache",
		Version: version,
		Features: engine.Features{
			Cache:          true,
			SpendTracking:  true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
