// ABRouter — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "ABRouter",
		Product: "abrouter",
		Version: version,
		Features: engine.Features{
			ABRouter:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
