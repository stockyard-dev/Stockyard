// LocalSync — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "LocalSync",
		Product: "localsync",
		Version: version,
		Features: engine.Features{
			LocalSync:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
