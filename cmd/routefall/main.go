// RouteFall — "Automatic LLM failover that just works."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "RouteFall",
		Product: "routefall",
		Version: version,
		Features: engine.Features{
			Failover:       true,
			RequestLogging: true,
			SpendTracking:  true,
		},
	})
}
