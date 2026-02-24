// RetryPilot — "Production-grade retries with circuit breaking and model downgrade."
package main

import "github.com/stockyard-dev/stockyard/internal/engine"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "RetryPilot",
		Product: "retrypilot",
		Version: version,
		Features: engine.Features{
			RetryPilot:     true,
			SpendTracking:  true,
			RequestLogging: true,
		},
	})
}
