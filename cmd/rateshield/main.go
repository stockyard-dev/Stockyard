// RateShield — "Protect your LLM endpoints from abuse."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "RateShield",
		Product: "rateshield",
		Version: version,
		Features: engine.Features{
			RateLimiting:   true,
			RequestLogging: true,
		},
	})
}
