// AgeGate — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "AgeGate",
		Product: "agegate",
		Version: version,
		Features: engine.Features{
			AgeGate:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
