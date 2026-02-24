// CodeFence — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "CodeFence",
		Product: "codefence",
		Version: version,
		Features: engine.Features{
			CodeFence:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
