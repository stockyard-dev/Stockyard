// SynthGen — Stockyard Phase 3 P3 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "SynthGen",
		Product: "synthgen",
		Version: version,
		Features: engine.Features{
			SynthGen:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
