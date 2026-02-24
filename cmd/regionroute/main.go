// RegionRoute — Stockyard Phase 3 P2 product.
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "RegionRoute",
		Product: "regionroute",
		Version: version,
		Features: engine.Features{
			RegionRoute:     true,
			RequestLogging: true,
			FullBodyLog:    true,
		},
	})
}
