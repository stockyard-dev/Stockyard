// ContextWindow — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ContextWindow",
		Product: "contextwindow",
		Version: version,
		Features: engine.Features{
			ContextWindow:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
