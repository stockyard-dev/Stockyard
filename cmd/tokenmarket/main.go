// TokenMarket — Stockyard Phase 3 P3 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "TokenMarket",
		Product: "tokenmarket",
		Version: version,
		Features: engine.Features{
			TokenMarket:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
