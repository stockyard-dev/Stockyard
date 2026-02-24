// GuardRail — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "GuardRail",
		Product: "guardrail",
		Version: version,
		Features: engine.Features{
			GuardRail:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
