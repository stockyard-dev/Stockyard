// ChainForge — Stockyard Phase 3 P3 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ChainForge",
		Product: "chainforge",
		Version: version,
		Features: engine.Features{
			ChainForge:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
